package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"go.opentelemetry.io/otel/trace"
)

type Span interface {
	End()
}

// Tracer creates a the workflow tracer.
func Tracer(ctx Context) *WorkflowTracer {
	return &WorkflowTracer{
		t: workflowtracer.Tracer(ctx),
	}
}

type WorkflowTracer struct {
	t *workflowtracer.WorkflowTracer
}

// Start starts a new span.
func (wt *WorkflowTracer) Start(ctx Context, name string, opts ...trace.SpanStartOption) (Context, Span) {
	ctx, span := wt.t.Start(ctx, name, opts...)

	return ctx, &span
}
