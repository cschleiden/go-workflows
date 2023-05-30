package workflowtracer

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"go.opentelemetry.io/otel/trace"
)

type tracerContextKeyType int

const tracerKey tracerContextKeyType = iota

func WithWorkflowTracer(ctx sync.Context, tracer *WorkflowTracer) sync.Context {
	return sync.WithValue(ctx, tracerKey, tracer)
}

func Tracer(ctx sync.Context) *WorkflowTracer {
	if tracer, ok := ctx.Value(tracerKey).(*WorkflowTracer); ok {
		return tracer
	}

	panic("no tracer in context")
}

type spanContextKeyType int

const spanKey spanContextKeyType = iota

func ContextWithSpan(ctx sync.Context, span trace.Span) sync.Context {
	return sync.WithValue(ctx, spanKey, span)
}

func SpanFromContext(ctx sync.Context) trace.Span {
	if span, ok := ctx.Value(spanKey).(trace.Span); ok {
		return span
	}

	return nil
}

type WorkflowTracer struct {
	parentSpan trace.Span
	tracer     trace.Tracer
}

func New(tracer trace.Tracer) *WorkflowTracer {
	return &WorkflowTracer{
		tracer: tracer,
	}
}

func (wt *WorkflowTracer) Start(ctx sync.Context, name string, opts ...trace.SpanStartOption) (sync.Context, Span) {
	state := workflowstate.WorkflowState(ctx)

	sctx := trace.ContextWithSpan(context.Background(), SpanFromContext(ctx))
	opts = append(opts, trace.WithTimestamp(state.Time()))
	sctx, span := wt.tracer.Start(sctx, name, opts...)

	if state.Replaying() {
		sctx = trace.ContextWithSpanContext(sctx, span.SpanContext())
		span = trace.SpanFromContext(sctx)
	}

	return ContextWithSpan(ctx, span), Span{span, state}
}

type Span struct {
	span  trace.Span
	state *workflowstate.WfState
}

func (s *Span) End() {
	if !s.state.Replaying() {
		// Only end the trace when we are not replaying
		s.span.End()
	}
}
