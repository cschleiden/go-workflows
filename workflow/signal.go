package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func NewSignalChannel(ctx sync.Context, name string) sync.Channel {
	wfState := workflowstate.WorkflowState(ctx)

	return wfState.GetSignalChannel(name)
}
