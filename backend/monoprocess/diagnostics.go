package monoprocess

import (
	"context"
	"errors"

	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/core"
)

var _ diag.Backend = (*monoprocessBackend)(nil)

func (b *monoprocessBackend) GetWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) (*diag.WorkflowInstanceRef, error) {
	if diagBackend, ok := b.Backend.(diag.Backend); ok {
		return diagBackend.GetWorkflowInstance(ctx, instance)
	}
	return nil, errors.New("not implemented")
}

func (b *monoprocessBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID, afterExecutionID string, count int) ([]*diag.WorkflowInstanceRef, error) {
	if diagBackend, ok := b.Backend.(diag.Backend); ok {
		return diagBackend.GetWorkflowInstances(ctx, afterInstanceID, afterExecutionID, count)
	}
	return nil, errors.New("not implemented")
}

func (b *monoprocessBackend) GetWorkflowTree(ctx context.Context, instance *core.WorkflowInstance) (*diag.WorkflowInstanceTree, error) {
	if diagBackend, ok := b.Backend.(diag.Backend); ok {
		return diagBackend.GetWorkflowTree(ctx, instance)
	}
	return nil, errors.New("not implemented")
}
