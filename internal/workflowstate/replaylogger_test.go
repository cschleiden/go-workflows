package workflowstate

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/core"
	log "github.com/cschleiden/go-workflows/internal/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func Test_ReplayLogger_With(t *testing.T) {
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := NewWorkflowState(i, slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New())

	with := wfState.Logger().With(slog.String("foo", "bar"))
	require.IsType(t, &replayHandler{}, with.Handler())
}

func Test_ReplayLogger_WithGroup(t *testing.T) {
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := NewWorkflowState(i, slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New())

	with := wfState.Logger().WithGroup("group_name")
	require.IsType(t, &replayHandler{}, with.Handler())
}

func Test_UpdateLoggerWithSpan(t *testing.T) {
	// Use a real tracer so spans have valid TraceID/SpanID
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	sc := span.SpanContext()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := NewWorkflowState(i, logger, tracer, clock.New())

	wfState.UpdateLoggerWithSpan(sc)

	// Log a message and verify it contains trace context
	wfState.Logger().Info("test message")

	output := buf.String()
	require.Contains(t, output, log.TraceIDKey)
	require.Contains(t, output, log.SpanIDKey)
	require.Contains(t, output, sc.TraceID().String())
	require.Contains(t, output, sc.SpanID().String())
}

func Test_UpdateLoggerWithSpan_PreservesReplayHandler(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	sc := span.SpanContext()

	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := NewWorkflowState(i, slog.Default(), tracer, clock.New())

	wfState.UpdateLoggerWithSpan(sc)

	// The handler should still be a replayHandler after updating
	require.IsType(t, &replayHandler{}, wfState.Logger().Handler())
}
