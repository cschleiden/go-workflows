package tracing

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/sync"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var propagator propagation.TextMapPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

func InjectSpan(ctx context.Context, metadata *core.WorkflowMetadata) {
	propagator.Inject(ctx, metadata)
}

func ExtractSpan(ctx context.Context, metadata *core.WorkflowMetadata) context.Context {
	return propagator.Extract(ctx, metadata)
}

type traceContextKeyType int

const currentSpanKey traceContextKeyType = iota

func WorkflowContextWithSpan(ctx sync.Context, span trace.Span) sync.Context {
	return sync.WithValue(ctx, currentSpanKey, span)
}

func SpanFromWorkflowContext(ctx sync.Context) trace.Span {
	if span, ok := ctx.Value(currentSpanKey).(trace.Span); ok {
		return span
	}

	panic("no span in context")
}

type TracingContextPropagator struct {
}

var _ contextpropagation.ContextPropagator = &TracingContextPropagator{}

func (*TracingContextPropagator) Inject(ctx context.Context, metadata *core.WorkflowMetadata) error {
	InjectSpan(ctx, metadata)
	return nil
}

func (*TracingContextPropagator) Extract(ctx context.Context, metadata *core.WorkflowMetadata) (context.Context, error) {
	return ExtractSpan(ctx, metadata), nil
}

func (*TracingContextPropagator) InjectFromWorkflow(ctx sync.Context, metadata *core.WorkflowMetadata) error {
	// Ignore

	return nil
}

func (*TracingContextPropagator) ExtractToWorkflow(ctx sync.Context, metadata *core.WorkflowMetadata) (sync.Context, error) {
	// Ignore

	return ctx, nil
}
