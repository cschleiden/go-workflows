package workflow

import (
	"github.com/cschleiden/go-workflows/internal/signals"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
)

func NewSignalChannel[T any](ctx Context, name string) Channel[T] {
	wfState := workflowstate.WorkflowState(ctx)
	return workflowstate.GetSignalChannel[T](ctx, wfState, name)
}

func SignalWorkflow[T any](ctx Context, instanceID string, name string, arg T) Future[any] {
	ctx, span := workflowtracer.Tracer(ctx).Start(ctx, "SignalWorkflow")
	defer span.End()

	var a *signals.Activities
	return ExecuteActivity[any](ctx, ActivityOptions{
		RetryOptions: RetryOptions{
			MaxAttempts: 1,
		},
	}, a.DeliverWorkflowSignal, instanceID, name, arg)
}
