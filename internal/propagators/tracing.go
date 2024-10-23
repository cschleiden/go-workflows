package propagators

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var propagator propagation.TextMapPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

func injectSpan(ctx context.Context, metadata *workflow.Metadata) {
	propagator.Inject(ctx, metadata)
}

func extractSpan(ctx context.Context, metadata *workflow.Metadata) context.Context {
	return propagator.Extract(ctx, metadata)
}

type TracingContextPropagator struct {
}

var _ workflow.ContextPropagator = &TracingContextPropagator{}

func (*TracingContextPropagator) Inject(ctx context.Context, metadata *workflow.Metadata) error {
	injectSpan(ctx, metadata)
	return nil
}

func (*TracingContextPropagator) Extract(ctx context.Context, metadata *workflow.Metadata) (context.Context, error) {
	return extractSpan(ctx, metadata), nil
}

func (*TracingContextPropagator) InjectFromWorkflow(ctx workflow.Context, metadata *workflow.Metadata) error {
	span := tracing.SpanFromContext(ctx)
	sctx := trace.ContextWithSpan(context.Background(), span)

	injectSpan(sctx, metadata)

	return nil
}

func (*TracingContextPropagator) ExtractToWorkflow(ctx workflow.Context, metadata *workflow.Metadata) (workflow.Context, error) {
	sctx := extractSpan(context.Background(), metadata)
	span := trace.SpanFromContext(sctx)

	return tracing.ContextWithSpan(ctx, span), nil
}
