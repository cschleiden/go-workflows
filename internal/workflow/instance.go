package workflow

import (
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/sync"
)

func WorkflowInstance2(ctx sync.Context) core.WorkflowInstance {
	wfState := getWfState(ctx)
	return wfState.instance
}
