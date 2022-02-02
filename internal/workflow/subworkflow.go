package workflow

import (
	a "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

type SubWorkflowInstanceOptions struct {
	InstanceID string
}

func CreateSubWorkflowInstance(ctx sync.Context, options SubWorkflowInstanceOptions, workflow Workflow, args ...interface{}) sync.Future {
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
	command := command.NewScheduleSubWorkflowCommand(eventID, options.InstanceID, name, inputs)
	wfState.addCommand(command)

	wfState.pendingFutures[eventID] = f

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.ReceiveNonBlocking(ctx, func(_ interface{}) {
				wfState.removeCommand(command)
				delete(wfState.pendingFutures, eventID)
				f.Set(nil, sync.Canceled)
			})
		}
	}

	return f
}
