package workflow

import (
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func SideEffect[TResult any](ctx sync.Context, f func(ctx sync.Context) TResult) Future[TResult] {
	wfState := workflowstate.WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	future := sync.NewFuture[TResult]()

	if Replaying(ctx) {
		// There has to be a message in the history with the result, create a new future
		// and block on it
		wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(future))

		return future
	}

	// Execute side effect
	r := f(ctx)

	// Create command to add it to the history
	payload, err := converter.DefaultConverter.To(r)
	if err != nil {
		var z TResult
		future.Set(z, err)
	}

	cmd := command.NewSideEffectCommand(scheduleEventID, payload)
	wfState.AddCommand(&cmd)

	future.Set(r, nil)

	return future
}
