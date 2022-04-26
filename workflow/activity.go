package workflow

import (
	"fmt"

	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

type ActivityOptions struct {
	RetryOptions RetryOptions
}

var DefaultActivityOptions = ActivityOptions{
	RetryOptions: DefaultRetryOptions,
}

// ExecuteActivity schedules the given activity to be executed
func ExecuteActivity[TResult any](ctx sync.Context, options ActivityOptions, activity interface{}, args ...interface{}) Future[TResult] {
	return withRetries(ctx, options.RetryOptions, func(ctx sync.Context) Future[TResult] {
		return executeActivity[TResult](ctx, options, activity, args...)
	})
}

func executeActivity[TResult any](ctx sync.Context, options ActivityOptions, activity interface{}, args ...interface{}) Future[TResult] {
	f := sync.NewFuture[TResult]()

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(*new(TResult), fmt.Errorf("converting activity input: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	name := fn.Name(activity)
	cmd := command.NewScheduleActivityTaskCommand(scheduleEventID, name, inputs)
	wfState.AddCommand(&cmd)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(f))

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal[struct{}]); ok {
			if _, ok := c.ReceiveNonBlocking(ctx); ok {
				// Workflow has been canceled, check if the activity has already been scheduled, no need to schedule otherwise
				if cmd.State != command.CommandState_Committed {
					wfState.RemoveCommand(&cmd)

					wfState.RemoveFuture(scheduleEventID)

					f.Set(*new(TResult), sync.Canceled)
				}
			}
		}
	}

	return f
}
