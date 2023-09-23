package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

// WorkflowInstance returns the current workflow instance.
func WorkflowInstance(ctx Context) *Instance {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Instance()
}
