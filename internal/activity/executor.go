package activity

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/pkg/core/task"
	"github.com/cschleiden/go-workflows/pkg/history"
	"github.com/pkg/errors"
)

type Executor struct {
	r *workflow.Registry
}

func NewExecutor(r *workflow.Registry) Executor {
	return Executor{
		r: r,
	}
}
func (e *Executor) ExecuteActivity(ctx context.Context, task *task.Activity) (payload.Payload, error) {
	a := task.Event.Attributes.(*history.ActivityScheduledAttributes)

	activity, err := e.r.GetActivity(a.Name)
	if err != nil {
		return nil, errors.Wrap(err, "could not find activity in registry")
	}

	activityFn := reflect.ValueOf(activity)
	if activityFn.Type().Kind() != reflect.Func {
		return nil, errors.New("activity not a function")
	}

	args, err := args.InputsToArgs(converter.DefaultConverter, activityFn, a.Inputs)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert activity inputs")
	}

	if !args[0].IsValid() {
		args[0] = reflect.ValueOf(ctx)
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
			return nil, errors.Wrap(err, "could not convert activity result")
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
