package backend

import (
	"context"

	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Backend interface {
	// CreateWorkflowInstance creates a new workflow instance
	CreateWorkflowInstance(context.Context, core.TaskMessage) error

	// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
	GetWorkflowTask(context.Context) (*task.Workflow, error)

	// CompleteWorkflowTask completes a workflow task retrieved using GetWorkflowTask
	//
	// This checkpoints the execution and schedules any new commands.
	CompleteWorkflowTask(ctx context.Context, task task.Workflow, newEvents []history.HistoryEvent) error

	// GetActivityTask returns a pending activity task or nil if there are no pending activities
	GetActivityTask(context.Context) (*task.Activity, error)

	// CompleteActivityTask completes a activity task retrieved using GetActivityTask
	CompleteActivityTask(context.Context, core.WorkflowInstance, string, history.HistoryEvent) error // TODO: Name TaskID?
}
