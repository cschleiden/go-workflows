package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

// WorkflowInstance returns the current workflow instance.
func WorkflowInstance(ctx Context) *Instance {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Instance()
}

// WorkflowInstanceExecutionDetails contains information about the current workflow execution.
type WorkflowInstanceExecutionDetails struct {
	// HistoryLength is the number of events in the workflow history at the current point in execution.
	// This value increases as the workflow executes and generates new events.
	HistoryLength int64
}

// InstanceExecutionDetails returns information about the current workflow execution.
func InstanceExecutionDetails(ctx Context) WorkflowInstanceExecutionDetails {
	wfState := workflowstate.WorkflowState(ctx)

	return WorkflowInstanceExecutionDetails{
		HistoryLength: wfState.HistoryLength(),
	}
}
