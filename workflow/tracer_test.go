package workflow

import (
	"context"
	"log/slog"
	"testing"

	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowstate"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func newTestContext(tracer trace.Tracer) (sync.Context, *workflowstate.WfState) {
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := workflowstate.NewWorkflowState(i, slog.Default(), tracer, clock.New())

	ctx := sync.Background()
	ctx = workflowstate.WithWorkflowState(ctx, wfState)
	return ctx, wfState
}

func TestSpanFromContext_NoSpan(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	ctx, _ := newTestContext(tracer)

	span := SpanFromContext(ctx)
	require.Nil(t, span)
}

func TestSpanFromContext_WithSpan(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	ctx, _ := newTestContext(tracer)

	// Start a real span and put it in the workflow context
	_, otelSpan := tracer.Start(context.Background(), "test-span")
	ctx = tracing.ContextWithSpan(ctx, otelSpan)

	span := SpanFromContext(ctx)
	require.NotNil(t, span)

	sc := span.SpanContext()
	require.True(t, sc.TraceID().IsValid())
	require.True(t, sc.SpanID().IsValid())
}

func TestSpan_SpanContext(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	_, wfState := newTestContext(tracer)

	// Create a real span
	_, otelSpan := tracer.Start(context.Background(), "test-span")
	expectedSC := otelSpan.SpanContext()

	s := &wfSpan{span: otelSpan, state: wfState}

	// Verify SpanContext() returns the same data
	sc := s.SpanContext()
	require.Equal(t, expectedSC.TraceID(), sc.TraceID())
	require.Equal(t, expectedSC.SpanID(), sc.SpanID())

	// Verify the Span interface is satisfied
	var _ Span = s
	require.True(t, sc.TraceID().IsValid())
	require.True(t, sc.SpanID().IsValid())
}

func TestSpan_End_NotReplaying(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	_, wfState := newTestContext(tracer)
	wfState.SetReplaying(false)

	_, otelSpan := tracer.Start(context.Background(), "test-span")
	s := &wfSpan{span: otelSpan, state: wfState}

	// Should not panic
	s.End()
}

func TestSpan_End_Replaying(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	_, wfState := newTestContext(tracer)
	wfState.SetReplaying(true)

	_, otelSpan := tracer.Start(context.Background(), "test-span")
	s := &wfSpan{span: otelSpan, state: wfState}

	// Should not panic; span.End() should be skipped during replay
	s.End()
}
