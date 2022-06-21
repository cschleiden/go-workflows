package tracing

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/sync"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var propagator propagation.TextMapPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

func MarshalSpan(ctx context.Context, metadata *core.WorkflowMetadata) {
	propagator.Inject(ctx, metadata)
}

func UnmarshalSpan(ctx context.Context, metadata *core.WorkflowMetadata) context.Context {
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
