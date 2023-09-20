package tracing

import (
	"context"

	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var propagator propagation.TextMapPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

func injectSpan(ctx context.Context, metadata *metadata.WorkflowMetadata) {
	propagator.Inject(ctx, metadata)
}

func extractSpan(ctx context.Context, metadata *metadata.WorkflowMetadata) context.Context {
	return propagator.Extract(ctx, metadata)
}

type TracingContextPropagator struct {
}

var _ contextpropagation.ContextPropagator = &TracingContextPropagator{}

func (*TracingContextPropagator) Inject(ctx context.Context, metadata *metadata.WorkflowMetadata) error {
	injectSpan(ctx, metadata)
	return nil
}

func (*TracingContextPropagator) Extract(ctx context.Context, metadata *metadata.WorkflowMetadata) (context.Context, error) {
	return extractSpan(ctx, metadata), nil
}

func (*TracingContextPropagator) InjectFromWorkflow(ctx sync.Context, metadata *metadata.WorkflowMetadata) error {
	span := workflowtracer.SpanFromContext(ctx)
	sctx := trace.ContextWithSpan(context.Background(), span)

	injectSpan(sctx, metadata)

	return nil
}

func (*TracingContextPropagator) ExtractToWorkflow(ctx sync.Context, metadata *metadata.WorkflowMetadata) (sync.Context, error) {
	sctx := extractSpan(context.Background(), metadata)
	span := trace.SpanFromContext(sctx)

	return workflowtracer.ContextWithSpan(ctx, span), nil
}
