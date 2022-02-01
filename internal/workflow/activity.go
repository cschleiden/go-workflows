package workflow

import (
	a "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

type Activity interface{}

// ExecuteActivity schedules the given activity to be executed
func ExecuteActivity(ctx sync.Context, activity Activity, args ...interface{}) sync.Future {
	f := sync.NewFuture()

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(nil, errors.Wrap(err, "failed to convert activity input"))
		return f
	}

	wfState := getWfState(ctx)
	eventID := wfState.eventID
	wfState.eventID++

	name := fn.Name(activity)
	command := command.NewScheduleActivityTaskCommand(eventID, name, inputs)
	wfState.addCommand(command)

	wfState.pendingFutures[eventID] = f

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			ok := c.ReceiveNonBlocking(ctx, func(_ interface{}) {
				wfState.removeCommand(command)
				delete(wfState.pendingFutures, eventID)
				f.Set(nil, sync.Canceled)
			})
			if !ok {
				// callback added
			}
		}
	}

	return f
}
