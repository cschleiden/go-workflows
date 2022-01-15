package workflow

import (
	a "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

func CreateSubWorkflowInstance(ctx sync.Context, workflow Workflow, args ...interface{}) sync.Future {
	f := sync.NewFuture()

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(nil, errors.Wrap(err, "failed to convert workflow input"))
		return f
	}

	wfState := getWfState(ctx)

	eventID := wfState.eventID
	wfState.eventID++

	name := fn.Name(workflow)
	command := command.NewScheduleSubWorkflowCommand(eventID, name, inputs)
	wfState.addCommand(command)

	wfState.pendingFutures[eventID] = f

	return f
}
