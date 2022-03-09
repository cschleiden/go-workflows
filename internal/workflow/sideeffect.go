package workflow

import (
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/sync"
)

func SideEffect(ctx sync.Context, f func(ctx sync.Context) interface{}) sync.Future {
	wfState := WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	future := sync.NewFuture()

	if Replaying(ctx) {
		// There has to be a message in the history with the result, create a new future
		// and block on it
		wfState.pendingFutures[scheduleEventID] = future

		return future
	}

	// Execute side effect
	r := f(ctx)

	// Create command to add it to the history
	payload, err := converter.DefaultConverter.To(r)
	if err != nil {
		future.Set(nil, err)
	}

	cmd := command.NewSideEffectCommand(scheduleEventID, payload)
	wfState.AddCommand(&cmd)

	future.Set(r, nil)

	return future
}
