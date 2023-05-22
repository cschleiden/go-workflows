package workflow

import (
	"fmt"

	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"github.com/cschleiden/go-workflows/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type SubWorkflowOptions struct {
	InstanceID string

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

func CreateSubWorkflowInstance[TResult any](ctx sync.Context, options SubWorkflowOptions, workflow interface{}, args ...interface{}) Future[TResult] {
	return withRetries(ctx, options.RetryOptions, func(ctx sync.Context, attempt int) Future[TResult] {
		return createSubWorkflowInstance[TResult](ctx, options, attempt, workflow, args...)
	})
}

func createSubWorkflowInstance[TResult any](ctx sync.Context, options SubWorkflowOptions, attempt int, wf interface{}, args ...interface{}) Future[TResult] {
	f := sync.NewFuture[TResult]()

	// If the context is already canceled, return immediately.
	if ctx.Err() != nil {
		f.Set(*new(TResult), ctx.Err())
		return f
	}

	// Check return type
	if err := a.ReturnTypeMatch[TResult](wf); err != nil {
		f.Set(*new(TResult), err)
		return f
	}

	// Check arguments
	if err := a.ParamsMatch(wf, args...); err != nil {
		f.Set(*new(TResult), err)
		return f
	}

	name := fn.Name(wf)

	cv := converter.GetConverter(ctx)
	inputs, err := a.ArgsToInputs(cv, args...)
	if err != nil {
		f.Set(*new(TResult), fmt.Errorf("converting subworkflow input: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	ctx, span := workflowtracer.Tracer(ctx).Start(ctx,
		fmt.Sprintf("CreateSubworkflowInstance: %s", name),
		trace.WithAttributes(
			attribute.String(log.WorkflowNameKey, name),
			attribute.Int64(log.ScheduleEventIDKey, scheduleEventID),
			attribute.Int(log.AttemptKey, attempt),
		))
	defer span.End()

	metadata := &core.WorkflowMetadata{}
	span.Marshal(metadata)

	cmd := command.NewScheduleSubWorkflowCommand(scheduleEventID, wfState.Instance(), options.InstanceID, name, inputs, metadata)

	wfState.AddCommand(cmd)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(cv, f))

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
							f.Set(*new(TResult), sync.Canceled)
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
