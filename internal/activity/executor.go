package activity

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Executor struct {
	logger log.Logger
	tracer trace.Tracer
	r      *workflow.Registry
}

func NewExecutor(logger log.Logger, tracer trace.Tracer, r *workflow.Registry) Executor {
	return Executor{
		logger: logger,
		tracer: tracer,
		r:      r,
	}
}

func (e *Executor) ExecuteActivity(ctx context.Context, task *task.Activity) (payload.Payload, error) {
	a := task.Event.Attributes.(*history.ActivityScheduledAttributes)

	activity, err := e.r.GetActivity(a.Name)
	if err != nil {
		return nil, err
	}

	activityFn := reflect.ValueOf(activity)
	if activityFn.Type().Kind() != reflect.Func {
		return nil, errors.New("activity not a function")
	}

	args, addContext, err := args.InputsToArgs(converter.DefaultConverter, activityFn, a.Inputs)
	if err != nil {
		return nil, fmt.Errorf("converting activity inputs: %w", err)
	}

	// Add activity state to context
	as := NewActivityState(
		task.Event.ID,
		task.WorkflowInstance,
		e.logger)
	activityCtx := WithActivityState(ctx, as)

	activityCtx = tracing.UnmarshalSpan(activityCtx, task.Metadata)
	activityCtx, span := e.tracer.Start(activityCtx, "ActivityTaskExecution", trace.WithAttributes(
		attribute.String("activity", a.Name),
		attribute.String(tracing.WorkflowInstanceID, task.WorkflowInstance.InstanceID),
		attribute.String(tracing.ActivityTaskID, task.ID),
	))
	defer span.End()

	// Execute activity
	if addContext {
		args[0] = reflect.ValueOf(activityCtx)
	}
	r := activityFn.Call(args)

	if len(r) < 1 || len(r) > 2 {
		return nil, errors.New("activity has to return either (error) or (<result>, error)")
	}

	var result payload.Payload

	if len(r) > 1 {
		var err error
		result, err = converter.DefaultConverter.To(r[0].Interface())
		if err != nil {
			return nil, fmt.Errorf("converting activity result: %w", err)
		}
	}

	errResult := r[len(r)-1]
	if errResult.IsNil() {
		return result, nil
	}

	errInterface, ok := errResult.Interface().(error)
	if !ok {
		return nil, fmt.Errorf("activity error result does not satisfy error interface (%T): %v", errResult, errResult)
	}

	return result, errInterface
}

func executeActivity(fn reflect.Value, args []reflect.Value) (r []reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {

		}
	}()

	r = fn.Call(args)
	if len(r) < 1 || len(r) > 2 {
		return nil, errors.New("activity has to return either (error) or (<result>, error)")
	}

	return r, nil
}
