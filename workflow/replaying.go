package workflow

import (
	"github.com/ticctech/go-workflows/internal/sync"
	"github.com/ticctech/go-workflows/internal/workflowstate"
)

func Replaying(ctx sync.Context) bool {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Replaying()
}
