package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

// Replaying returns true if the current workflow execution is replaying or not.
func Replaying(ctx Context) bool {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Replaying()
}
