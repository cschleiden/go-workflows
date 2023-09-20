package contextpropagation

import (
	"context"

	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/internal/sync"
)

type ContextPropagator interface {
	Inject(context.Context, *metadata.WorkflowMetadata) error
	Extract(context.Context, *metadata.WorkflowMetadata) (context.Context, error)

	InjectFromWorkflow(sync.Context, *metadata.WorkflowMetadata) error
	ExtractToWorkflow(sync.Context, *metadata.WorkflowMetadata) (sync.Context, error)
}

func Inject(ctx context.Context, metadata *metadata.WorkflowMetadata, propagators []ContextPropagator) error {
	for _, propagator := range propagators {
		err := propagator.Inject(ctx, metadata)
		if err != nil {
			return err
		}
	}

	return nil
}

func Extract(ctx context.Context, metadata *metadata.WorkflowMetadata, propagators []ContextPropagator) (context.Context, error) {
	for _, propagator := range propagators {
		var err error
		ctx, err = propagator.Extract(ctx, metadata)
		if err != nil {
			return nil, err
		}
	}

	return ctx, nil
}

func InjectFromWorkflow(ctx sync.Context, metadata *metadata.WorkflowMetadata, propagators []ContextPropagator) error {
	for _, propagator := range propagators {
		err := propagator.InjectFromWorkflow(ctx, metadata)
		if err != nil {
			return err
		}
	}

	return nil
}

func ExtractToWorkflow(ctx sync.Context, metadata *metadata.WorkflowMetadata, propagators []ContextPropagator) (sync.Context, error) {
	for _, propagator := range propagators {
		var err error
		ctx, err = propagator.ExtractToWorkflow(ctx, metadata)
		if err != nil {
			return nil, err
		}
	}

	return ctx, nil
}

type propagatorsKey int

var propagatorsCtxKey propagatorsKey

func WithPropagators(ctx sync.Context, propagators []ContextPropagator) sync.Context {
	return sync.WithValue(ctx, propagatorsCtxKey, propagators)
}

func Propagators(ctx sync.Context) []ContextPropagator {
	propagators, ok := ctx.Value(propagatorsCtxKey).([]ContextPropagator)
	if !ok {
		return nil
	}

	return propagators
}
