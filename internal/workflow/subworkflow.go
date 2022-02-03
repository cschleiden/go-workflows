package workflow

import (
	a "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

type SubWorkflowOptions struct {
	InstanceID string

	RetryOptions RetryOptions
}

func CreateSubWorkflowInstance(ctx sync.Context, options SubWorkflowOptions, workflow Workflow, args ...interface{}) sync.Future {
	return WithRetries(ctx, options.RetryOptions, func(ctx sync.Context) sync.Future {
		return createSubWorkflowInstance(ctx, options, workflow, args...)
	})
}

func createSubWorkflowInstance(ctx sync.Context, options SubWorkflowOptions, workflow Workflow, args ...interface{}) sync.Future {
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
	cmd := command.NewScheduleSubWorkflowCommand(eventID, options.InstanceID, name, inputs)
	wfState.addCommand(&cmd)

	wfState.pendingFutures[eventID] = f

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.ReceiveNonBlocking(ctx, func(_ interface{}) {
				// Workflow has been canceled, check if the sub-workflow has already been scheduled
				if cmd.State == command.CommandState_Committed {
					// Command has already been committed, that means the sub-workflow has already been scheduled. Wait
					// until it is done.
					return
				}

				wfState.removeCommand(cmd)
				delete(wfState.pendingFutures, eventID)
				f.Set(nil, sync.Canceled)
			})
		}
	}

	return f
}
