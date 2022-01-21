package workflow

import (
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
)

func SetTime(ctx sync.Context, t time.Time) {
	wfState := getWfState(ctx)
	wfState.time = t
}

func Now(ctx sync.Context) time.Time {
	wfState := getWfState(ctx)
	return wfState.time
}
