package workflow

import "github.com/cschleiden/go-workflows/internal/sync"

func Replaying(ctx sync.Context) bool {
	wfState := WorkflowState(ctx)
	return wfState.replaying
}

func SetReplaying(ctx sync.Context, replaying bool) {
	wfState := WorkflowState(ctx)
	wfState.replaying = replaying
}
