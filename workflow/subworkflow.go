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

type SubWorkflowOptions struct {
	InstanceID string

	RetryOptions RetryOptions
}

var DefaultSubWorkflowOptions = SubWorkflowOptions{
	RetryOptions: DefaultRetryOptions,
}

func CreateSubWorkflowInstance[TResult any](ctx sync.Context, options SubWorkflowOptions, workflow interface{}, args ...interface{}) Future[TResult] {
	return withRetries(ctx, options.RetryOptions, func(ctx sync.Context) Future[TResult] {
		return createSubWorkflowInstance[TResult](ctx, options, workflow, args...)
	})
}

func createSubWorkflowInstance[TResult any](ctx sync.Context, options SubWorkflowOptions, workflow interface{}, args ...interface{}) Future[TResult] {
	f := sync.NewFuture[TResult]()

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		var z TResult
		f.Set(z, fmt.Errorf("converting workflow input: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	name := fn.Name(workflow)
	cmd := command.NewScheduleSubWorkflowCommand(scheduleEventID, options.InstanceID, name, inputs)
	wfState.AddCommand(&cmd)

	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(f))

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal[struct{}]); ok {
			if _, ok := c.ReceiveNonBlocking(ctx); ok {
				// Workflow has been canceled, check if the sub-workflow has already been scheduled
				if cmd.State != command.CommandState_Committed {
					wfState.RemoveCommand(cmd)
					wfState.RemoveFuture(scheduleEventID)
					var z TResult
					f.Set(z, sync.Canceled)
				}
			}
		}
	}

	return f
}
