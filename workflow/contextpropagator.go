package workflow

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/contextvalue"
)

type ContextPropagator interface {
	// Inject injects values from context into metadata
	Inject(context.Context, *Metadata) error

	// Extract extracts values from metadata into context
	Extract(context.Context, *Metadata) (context.Context, error)

	// InjectFromWorkflow injects values from the workflow context into metadata
	InjectFromWorkflow(Context, *Metadata) error

	// ExtractToWorkflow extracts values from metadata into a workflow context
	ExtractToWorkflow(Context, *Metadata) (Context, error)
}

func injectFromWorkflow(ctx Context, metadata *Metadata, propagators []ContextPropagator) error {
	for _, propagator := range propagators {
		err := propagator.InjectFromWorkflow(ctx, metadata)
		if err != nil {
			return err
		}
	}

	return nil
}

func propagators(ctx Context) []ContextPropagator {
	propagators, ok := ctx.Value(contextvalue.PropagatorsCtxKey).([]ContextPropagator)
	if !ok {
		return nil
	}

	return propagators
}
