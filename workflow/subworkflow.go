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

	if ctx.Err() != nil {
		f.Set(*new(TResult), ctx.Err())
		return f
	}

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(*new(TResult), fmt.Errorf("converting subworkflow input: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	name := fn.Name(workflow)
	cmd := command.NewScheduleSubWorkflowCommand(scheduleEventID, wfState.Instance(), options.InstanceID, name, inputs)
	wfState.AddCommand(&cmd)

	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(f))

	// Check if the channel is cancelable
	if c, cancelable := ctx.Done().(sync.CancelChannel); cancelable {
		c.AddReceiveCallback(func(v struct{}, ok bool) {
			if cmd.State == command.CommandState_Committed {
				// Sub-workflow is already started, create cancel command
				cancelScheduleEventID := wfState.GetNextScheduleEventID()

				a := cmd.Attr.(*command.ScheduleSubWorkflowCommandAttr)
				subworkflowCancellationCmd := command.NewCancelSubWorkflowCommand(cancelScheduleEventID, a.Instance)
				wfState.AddCommand(&subworkflowCancellationCmd)
			}

			// Remove the sub-workflow future from the workflow state and mark it as canceled if it hasn't already fired
			if fi, ok := f.(sync.FutureInternal[struct{}]); ok {
				if !fi.Ready() {
					wfState.RemoveFuture(scheduleEventID)
					f.Set(*new(TResult), sync.Canceled)
				}
			}
		})
	}

	return f
}
