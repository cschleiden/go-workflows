package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func Replaying(ctx Context) bool {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Replaying()
}
