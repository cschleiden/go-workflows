package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func Now(ctx sync.Context) time.Time {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Time()
}
