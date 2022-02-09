package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func Replaying(ctx sync.Context) bool {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Replaying()
}
