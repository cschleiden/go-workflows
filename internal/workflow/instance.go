package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/pkg/core"
)

func WorkflowInstance(ctx sync.Context) core.WorkflowInstance {
	wfState := getWfState(ctx)
	return wfState.instance
}
