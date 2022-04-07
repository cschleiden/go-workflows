package workflow

import (
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func WorkflowInstance(ctx sync.Context) *core.WorkflowInstance {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Instance()
}
