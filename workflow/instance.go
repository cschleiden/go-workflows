package workflow

import (
	"github.com/ticctech/go-workflows/internal/core"
	"github.com/ticctech/go-workflows/internal/sync"
	"github.com/ticctech/go-workflows/internal/workflowstate"
)

func WorkflowInstance(ctx sync.Context) *core.WorkflowInstance {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Instance()
}
