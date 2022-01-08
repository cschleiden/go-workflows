package workflow

import (
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

func CreateSubWorkflowInstance(ctx sync.Context, name string, args ...interface{}) (sync.Future, error) {
	wfState := getWfState(ctx)

	inputs, err := converter.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert activity input")
	}

	eventID := wfState.eventID
	wfState.eventID++

	command := command.NewScheduleSubWorkflowCommand(eventID, name, "", inputs)
	wfState.addCommand(command)

	f := sync.NewFuture()
	wfState.pendingFutures[eventID] = f

	return f, nil
}
