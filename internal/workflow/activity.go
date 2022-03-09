package workflow

import (
	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/pkg/errors"
)

type Activity interface{}

type ActivityOptions struct {
	RetryOptions RetryOptions
}

var DefaultActivityOptions = ActivityOptions{
	RetryOptions: DefaultRetryOptions,
}

// ExecuteActivity schedules the given activity to be executed
func ExecuteActivity(ctx sync.Context, options ActivityOptions, activity Activity, args ...interface{}) sync.Future {
	return WithRetries(ctx, options.RetryOptions, func(ctx sync.Context) sync.Future {
		return executeActivity(ctx, options, activity, args...)
	})
}

func executeActivity(ctx sync.Context, options ActivityOptions, activity Activity, args ...interface{}) sync.Future {
	f := sync.NewFuture()

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(nil, errors.Wrap(err, "failed to convert activity input"))
		return f
	}

	wfState := WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	name := fn.Name(activity)
	cmd := command.NewScheduleActivityTaskCommand(scheduleEventID, name, inputs)
	wfState.AddCommand(&cmd)

	wfState.pendingFutures[scheduleEventID] = f

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.ReceiveNonBlocking(ctx, func(_ interface{}) {
				// Workflow has been canceled, check if the activity has already been scheduled
				if cmd.State == command.CommandState_Committed {
					// Command has already been committed, that means the activity has already been scheduled. Wait
					// until the activity is done.
					return
				}

				wfState.RemoveCommand(cmd)
				delete(wfState.pendingFutures, scheduleEventID)
				f.Set(nil, sync.Canceled)
			})
		}
	}

	return f
}
