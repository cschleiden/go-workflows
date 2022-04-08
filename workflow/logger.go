package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/log"
)

func Logger(ctx Context) log.Logger {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Logger()
}
