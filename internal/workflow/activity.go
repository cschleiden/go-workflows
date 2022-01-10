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
func ExecuteActivity(ctx sync.Context, activity Activity, args ...interface{}) (sync.Future, error) {
	wfState := getWfState(ctx)

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert activity input")
	}

	eventID := wfState.eventID
	wfState.eventID++

	name := fn.Name(activity)
	command := command.NewScheduleActivityTaskCommand(eventID, name, inputs)
	wfState.addCommand(command)

	f := sync.NewFuture()
	wfState.pendingFutures[eventID] = f

	return f, nil
}
