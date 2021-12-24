package workflow

import "context"

func Replaying(ctx context.Context) bool {
	wfState := getWfState(ctx)
	return wfState.replaying
}

func SetReplaying(ctx context.Context, replaying bool) {
	wfState := getWfState(ctx)
	wfState.replaying = replaying
}
