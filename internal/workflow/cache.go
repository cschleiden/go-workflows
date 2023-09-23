package workflow

import (
	"context"

	"github.com/cschleiden/go-workflows/core"
)

type ExecutorCache interface {
	Store(ctx context.Context, instance *core.WorkflowInstance, workflow WorkflowExecutor) error
	Get(ctx context.Context, instance *core.WorkflowInstance) (WorkflowExecutor, bool, error)
	StartEviction(ctx context.Context)
}
