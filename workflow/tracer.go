package workflow

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"go.opentelemetry.io/otel/trace"
)

type wfSpan struct {
	span  trace.Span
	state *workflowstate.WfState
}

func (s *wfSpan) End() {
	if !s.state.Replaying() {
		// Only end the trace when we are not replaying
		s.span.End()
	}
}

type Span interface {
	// End ends the span.
	End()
}

// Tracer creates a the workflow tracer.
func Tracer(ctx Context) *WorkflowTracer {
	return &WorkflowTracer{}
}

type WorkflowTracer struct {
}

// Start starts a new span.
func (wt *WorkflowTracer) Start(ctx Context, name string, opts ...trace.SpanStartOption) (Context, Span) {
	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	var span *wfSpan

	cmd := command.NewStartTraceCommand(scheduleEventID)
	wfState.AddCommand(cmd)

	future := sync.NewFuture[[8]byte]()

	cv := contextvalue.Converter(ctx)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(cv, "startTrace", future))

	if wfState.Replaying() {
		// We need the spanID of the original span
		spanID, _ := future.Get(ctx)

		// Use original timestamp
		opts := append(opts, trace.WithTimestamp(Now(ctx)))
		ctx, span = start(ctx, name, opts...)

		// Update underlying span with new spanID
		tracing.SetSpanID(span.span, spanID)
	} else {
		ctx, span = start(ctx, name, opts...)

		// Declare as [8]byte to convert to payload
		var spanID [8]byte = span.span.SpanContext().SpanID()
		payload, err := cv.To(spanID)
		if err != nil {
			future.Set([8]byte{}, err)
		}
		cmd.SetSpanID(payload)

		// Update span so our tracking doesn't report it
		future.Set(spanID, nil)
		wfState.RemoveFuture(scheduleEventID)
	}

	return ctx, span
}

func start(ctx Context, name string, opts ...trace.SpanStartOption) (sync.Context, *wfSpan) {
	state := workflowstate.WorkflowState(ctx)
	tracer := state.Tracer()

	// Set correct parent span
	sctx := trace.ContextWithSpan(context.Background(), tracing.SpanFromContext(ctx))

	// Use workflow timestamp
	opts = append(opts, trace.WithTimestamp(state.Time()))
	_, span := tracer.Start(sctx, name, opts...)

	return tracing.ContextWithSpan(ctx, span), &wfSpan{span, state}
}
