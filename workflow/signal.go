package workflow

import (
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func NewSignalChannel[T any](ctx Context, name string) Channel[T] {
	wfState := workflowstate.WorkflowState(ctx)
	return workflowstate.GetSignalChannel[T](ctx, wfState, name)
}
