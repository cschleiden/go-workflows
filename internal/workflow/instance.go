package workflow

import (
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/sync"
)

func WorkflowInstance(ctx sync.Context) core.WorkflowInstance {
	wfState := getWfState(ctx)
	return wfState.instance
}
