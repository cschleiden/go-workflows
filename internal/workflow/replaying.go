package workflow

import "github.com/cschleiden/go-dt/internal/sync"

func Replaying(ctx sync.Context) bool {
	wfState := getWfState(ctx)
	return wfState.replaying
}

func SetReplaying(ctx sync.Context, replaying bool) {
	wfState := getWfState(ctx)
	wfState.replaying = replaying
}
