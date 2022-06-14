package tracing

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/sync"
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

func NewWorkflowTracer(tracer trace.Tracer) *WorkflowTracer {
	return &WorkflowTracer{
		tracer: tracer,
	}
}

func (wt *WorkflowTracer) UpdateExecution(span trace.Span) {
	wt.parentSpan = span
}

func (wt *WorkflowTracer) Start(name string, opts ...trace.SpanStartOption) trace.Span {
	ctx := trace.ContextWithSpan(context.Background(), wt.parentSpan)
	_, span := wt.tracer.Start(ctx, name, opts...)
	return span
}
