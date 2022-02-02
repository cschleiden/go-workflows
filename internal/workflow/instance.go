package workflow

import (
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/cschleiden/go-dt/pkg/core"
)

func WorkflowInstance(ctx sync.Context) core.WorkflowInstance {
	wfState := getWfState(ctx)
	return wfState.instance
}
