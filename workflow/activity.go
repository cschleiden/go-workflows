package workflow

import (
	"fmt"

	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
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

type ActivityOptions struct {
	RetryOptions RetryOptions
}

var DefaultActivityOptions = ActivityOptions{
	RetryOptions: DefaultRetryOptions,
}

// ExecuteActivity schedules the given activity to be executed
func ExecuteActivity[TResult any](ctx Context, options ActivityOptions, activity interface{}, args ...interface{}) Future[TResult] {
	return WithRetries(ctx, options.RetryOptions, func(ctx sync.Context, attempt int) Future[TResult] {
		return executeActivity[TResult](ctx, options, attempt, activity, args...)
	})
}

func executeActivity[TResult any](ctx Context, options ActivityOptions, attempt int, activity interface{}, args ...interface{}) Future[TResult] {
	f := sync.NewFuture[TResult]()

	if ctx.Err() != nil {
		f.Set(*new(TResult), ctx.Err())
		return f
	}

	// Check return type
	if err := a.ReturnTypeMatch[TResult](activity); err != nil {
		f.Set(*new(TResult), err)
		return f
	}

	// Check arguments
	if err := a.ParamsMatch(activity, args...); err != nil {
		f.Set(*new(TResult), err)
		return f
	}

	cv := converter.GetConverter(ctx)
	inputs, err := a.ArgsToInputs(cv, args...)
	if err != nil {
		f.Set(*new(TResult), fmt.Errorf("converting activity input: %w", err))
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	name := fn.FuncName(activity)

	// Capture context
	propagators := contextpropagation.Propagators(ctx)
	metadata := &core.WorkflowMetadata{}
	if err := contextpropagation.InjectFromWorkflow(ctx, metadata, propagators); err != nil {
		f.Set(*new(TResult), fmt.Errorf("injecting workflow context: %w", err))
		return f
	}

	cmd := command.NewScheduleActivityCommand(scheduleEventID, name, inputs, metadata)
	wfState.AddCommand(cmd)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(cv, f))

	ctx, span := workflowtracer.Tracer(ctx).Start(ctx,
		fmt.Sprintf("ExecuteActivity: %s", name),
		trace.WithAttributes(
			attribute.String(log.ActivityNameKey, name),
			attribute.Int64(log.ScheduleEventIDKey, scheduleEventID),
			attribute.Int(log.AttemptKey, attempt),
		))
	defer span.End()

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal[struct{}]); ok {
			if _, ok := c.ReceiveNonBlocking(); ok {
				// Workflow has been canceled, check if the activity has already been scheduled, no need to schedule otherwise
				if cmd.State() == command.CommandState_Pending {
					cmd.Done()
					wfState.RemoveFuture(scheduleEventID)
					f.Set(*new(TResult), sync.Canceled)
				}
			}
		}
	}

	return f
}
