package executor

import (
	"context"

	"github.com/cschleiden/go-workflows/core"
)

type ExecutorCache interface {
	Store(ctx context.Context, instance *core.WorkflowInstance, workflow WorkflowExecutor) error
	Evict(ctx context.Context, instance *core.WorkflowInstance) error
	Get(ctx context.Context, instance *core.WorkflowInstance) (WorkflowExecutor, bool, error)
	StartEviction(ctx context.Context)
}
