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
