package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

// WorkflowInstance returns the current workflow instance.
func WorkflowInstance(ctx Context) *Instance {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Instance()
}

// WorkflowInstanceInfo contains metadata about the current workflow instance.
type WorkflowInstanceInfo struct {
	// HistoryLength is the current length of the workflow history (number of events).
	// This is useful for deciding when to call ContinueAsNew to avoid hitting history size limits.
	HistoryLength int64
}

// GetWorkflowInstanceInfo returns metadata about the current workflow instance,
// including the history length which is useful for deciding when to call ContinueAsNew.
func GetWorkflowInstanceInfo(ctx Context) WorkflowInstanceInfo {
	wfState := workflowstate.WorkflowState(ctx)
	return WorkflowInstanceInfo{
		HistoryLength: wfState.HistoryLength(),
	}
}
