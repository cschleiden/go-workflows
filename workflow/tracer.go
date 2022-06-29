package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"go.opentelemetry.io/otel/trace"
)

type Span interface {
	End()
}

type WorkflowTracer interface {
	Start(ctx Context, name string, opts ...trace.SpanStartOption) (Context, Span)
}

func Tracer(ctx Context) WorkflowTracer {
	return &workflowTracer{
		t: workflowtracer.Tracer(ctx),
	}
}

type workflowTracer struct {
	t *workflowtracer.WorkflowTracer
}

func (wt *workflowTracer) Start(ctx Context, name string, opts ...trace.SpanStartOption) (Context, Span) {
	ctx, span := wt.t.Start(ctx, name, opts...)

	return ctx, &span
}
