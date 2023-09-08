package workflow

import (
	"log/slog"

	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func Logger(ctx Context) *slog.Logger {
	wfState := workflowstate.WorkflowState(ctx)
	return wfState.Logger()
}
