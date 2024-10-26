package workflow

import (
	"fmt"

	"github.com/cschleiden/go-workflows/backend/metadata"
	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/tracing"

	// "github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

type SubWorkflowOptions struct {
	InstanceID string

	// Queue to use for the sub-workflow, if not set, the queue of the calling workflow will be used.
	Queue Queue

	RetryOptions RetryOptions
}

var (
	DefaultSubWorkflowRetryOptions = RetryOptions{
		// Disable retries by default for sub-workflows
		MaxAttempts: 1,
	}

	DefaultSubWorkflowOptions = SubWorkflowOptions{
		RetryOptions: DefaultSubWorkflowRetryOptions,
	}
)

// CreateSubWorkflowInstance creates a new sub-workflow instance of the given workflow.
func CreateSubWorkflowInstance[TResult any](ctx Context, options SubWorkflowOptions, workflow Workflow, args ...any) Future[TResult] {
	return WithRetries(ctx, options.RetryOptions, func(ctx Context, attempt int) Future[TResult] {
		return createSubWorkflowInstance[TResult](ctx, options, attempt, workflow, args...)
	})
}

func createSubWorkflowInstance[TResult any](ctx Context, options SubWorkflowOptions, attempt int, wf Workflow, args ...any) Future[TResult] {
	f := sync.NewFuture[TResult]()

	// If the context is already canceled, return immediately.
	if ctx.Err() != nil {
		f.Set(*new(TResult), ctx.Err())
		return f
	}

	// Check return type
	var workflowName string
	if name, ok := wf.(string); ok {
		workflowName = name
	} else {
		workflowName = fn.Name(wf)

		if err := a.ReturnTypeMatch[TResult](wf); err != nil {
			f.Set(*new(TResult), err)
			return f
		}

		// Check arguments
		if err := a.ParamsMatch(wf, args...); err != nil {
			f.Set(*new(TResult), err)
			return f
		}
	}

	cv := contextvalue.Converter(ctx)
	inputs, err := a.ArgsToInputs(cv, args...)
	if err != nil {
		f.Set(*new(TResult), fmt.Errorf("converting subworkflow input: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	// Capture context
	propagators := propagators(ctx)
	metadata := &metadata.WorkflowMetadata{}
	if err := injectFromWorkflow(ctx, metadata, propagators); err != nil {
		f.Set(*new(TResult), fmt.Errorf("injecting workflow context: %w", err))
		return f
	}

	workflowSpanID := tracing.GetNewSpanIDWF(ctx)

	cmd := command.NewScheduleSubWorkflowCommand(
		scheduleEventID,
		wfState.Instance(),
		options.Queue,
		options.InstanceID,
		workflowName,
		inputs,
		metadata,
		workflowSpanID,
	)

	wfState.AddCommand(cmd)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(cv, fmt.Sprintf("subworkflow:%s", workflowName), f))

	// Check if the channel is cancelable
	if c, cancelable := ctx.Done().(sync.CancelChannel); cancelable {
		cancelReceiver := &sync.Receiver[struct{}]{
			Receive: func(v struct{}, ok bool) {
				cmd.Cancel()
				if cmd.State() == command.CommandState_Canceled {
					// Remove the sub-workflow future from the workflow state and mark it as canceled if it hasn't already fired
					if fi, ok := f.(sync.FutureInternal[TResult]); ok {
						if !fi.Ready() {
							wfState.RemoveFuture(scheduleEventID)
							f.Set(*new(TResult), Canceled)
						}
					}
				}
			},
		}

		c.AddReceiveCallback(cancelReceiver)

		cmd.WhenDone(func() {
			c.RemoveReceiveCallback(cancelReceiver)
		})
	}

	return f
}
