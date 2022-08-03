package workflow

import (
	"fmt"

	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type SubWorkflowOptions struct {
	InstanceID string

	RetryOptions RetryOptions
}

var DefaultSubWorkflowOptions = SubWorkflowOptions{
	RetryOptions: DefaultRetryOptions,
}

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

	name := fn.Name(wf)

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(*new(TResult), fmt.Errorf("converting subworkflow input: %w", err))
		return f
	}

	if err := fn.CheckReturn[TResult](wf); err != nil {
		f.Set(*new(TResult), fmt.Errorf("checking subworkflow result: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	ctx, span := workflowtracer.Tracer(ctx).Start(ctx,
		fmt.Sprintf("CreateSubworkflowInstance: %s", name),
		trace.WithAttributes(
			attribute.String("name", name),
			attribute.Int64(tracing.ScheduleEventID, scheduleEventID),
			attribute.Int("attempt", attempt),
		))
	defer span.End()

	metadata := &core.WorkflowMetadata{}
	span.Marshal(metadata)

	cmd := command.NewScheduleSubWorkflowCommand(scheduleEventID, wfState.Instance(), options.InstanceID, name, inputs, metadata)
	wfState.AddCommand(cmd)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(f))

	// Check if the channel is cancelable
	if c, cancelable := ctx.Done().(sync.CancelChannel); cancelable {
		c.AddReceiveCallback(func(v struct{}, ok bool) {
			if cmd.Committed() {
				// The command is committed, that means the sub-workflow is already started. Create and add a cancel command
				// to stop the sub-workflow execution.
				cancelScheduleEventID := wfState.GetNextScheduleEventID()

				subworkflowCancellationCmd := command.NewCancelSubWorkflowCommand(cancelScheduleEventID, cmd.Instance)
				wfState.AddCommand(subworkflowCancellationCmd)
			} else {
				// Remove command that would've started the sub-workflow
				cmd.Done()

				// Remove the sub-workflow future from the workflow state and mark it as canceled if it hasn't already fired
				if fi, ok := f.(sync.FutureInternal[TResult]); ok {
					if !fi.Ready() {
						wfState.RemoveFuture(scheduleEventID)
						f.Set(*new(TResult), sync.Canceled)
					}
				}
			}
		})
	}

	return f
}
