package backend

import (
	"context"

	core "github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/workflow"
)

type WorkflowState int

const (
	WorkflowStateActive WorkflowState = iota
	WorkflowStateFinished
)

//go:generate mockery --name=Backend --inpackage
type Backend interface {
	// CreateWorkflowInstance creates a new workflow instance
	CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error

	// CancelWorkflowInstance cancels a running workflow instance
	CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error

	// GetWorkflowInstanceState returns the state of the given workflow instance
	GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (WorkflowState, error)

	// GetWorkflowInstanceHistory returns the full workflow history for the given instance
	GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance) ([]history.Event, error)

	// SignalWorkflow signals a running workflow instance
	SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error

	// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
	GetWorkflowTask(ctx context.Context) (*task.Workflow, error)

	// ExtendWorkflowTask extends the lock of a workflow task
	ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error

	// CompleteWorkflowTask checkpoints a workflow task retrieved using GetWorkflowTask
	//
	// This checkpoints the execution. events are new events from the last workflow execution
	// which will be added to the workflow instance history. workflowEvents are new events for the
	// completed or other workflow instances.
	CompleteWorkflowTask(
		ctx context.Context, taskID string, instance *workflow.Instance, state WorkflowState,
		executedEvents []history.Event, activityEvents []history.Event, workflowEvents []history.WorkflowEvent) error

	// GetActivityTask returns a pending activity task or nil if there are no pending activities
	GetActivityTask(ctx context.Context) (*task.Activity, error)

	// CompleteActivityTask completes a activity task retrieved using GetActivityTask
	CompleteActivityTask(ctx context.Context, instance *workflow.Instance, activityID string, event history.Event) error

	// ExtendActivityTask extends the lock of an activity task
	ExtendActivityTask(ctx context.Context, activityID string) error
}
