package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func WorkflowInstance(ctx Context) *Instance {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Instance()
}
