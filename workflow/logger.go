package workflow

import (
	"github.com/ticctech/go-workflows/internal/workflowstate"
	"github.com/ticctech/go-workflows/log"
)

func Logger(ctx Context) log.Logger {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Logger()
}
