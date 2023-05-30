package workflow

import "context"

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
