package activity

import (
	"context"

	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Executor struct {
	r *workflow.Registry
}

func NewExecutor(r *workflow.Registry) Executor {
	return Executor{
		r: r,
	}
}
func (e *Executor) ExecuteActivity(ctx context.Context, task task.Activity) (interface{}, error) {
	a := task.Event.Attributes.(history.ActivityScheduledAttributes)

	activity := e.r.GetActivity(a.Name)
	activityFn := activity.(func(context.Context) (interface{}, error)) // TODO: Activity inputs

	// TODO: Handle errors & panics
	result, _ := activityFn(ctx)

	return result, nil
}
