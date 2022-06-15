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

type WorkflowTracer struct {
	parentSpan trace.Span
	tracer     trace.Tracer
}

func New(tracer trace.Tracer) *WorkflowTracer {
	return &WorkflowTracer{
		tracer: tracer,
	}
}

func (wt *WorkflowTracer) UpdateExecution(span trace.Span) {
	wt.parentSpan = span
}

func (wt *WorkflowTracer) Start(ctx sync.Context, name string, opts ...trace.SpanStartOption) trace.Span {
	sctx := trace.ContextWithSpan(context.Background(), wt.parentSpan)
	sctx, span := wt.tracer.Start(sctx, name, opts...)

	state := workflowstate.WorkflowState(ctx)
	if state.Replaying() {
		sctx = trace.ContextWithSpanContext(sctx, span.SpanContext())
		return trace.SpanFromContext(sctx)
	}

	return span
}
