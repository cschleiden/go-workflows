package workflow

import (
	"github.com/ticctech/go-workflows/internal/command"
	"github.com/ticctech/go-workflows/internal/converter"
	"github.com/ticctech/go-workflows/internal/sync"
	"github.com/ticctech/go-workflows/internal/workflowstate"
	"github.com/ticctech/go-workflows/internal/workflowtracer"
)

func SideEffect[TResult any](ctx Context, f func(ctx Context) TResult) Future[TResult] {
	ctx, span := workflowtracer.Tracer(ctx).Start(ctx, "SideEffect")
	defer span.End()

	future := sync.NewFuture[TResult]()

	if ctx.Err() != nil {
		future.Set(*new(TResult), ctx.Err())
		return future
	}

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

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
		future.Set(*new(TResult), err)
	}

	cmd := command.NewSideEffectCommand(scheduleEventID, payload)
	wfState.AddCommand(cmd)

	future.Set(r, nil)

	return future
}
