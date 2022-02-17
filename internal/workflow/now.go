package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/sync"
)

func Now(ctx sync.Context) time.Time {
	wfState := getWfState(ctx)
	return wfState.time
}
