package workflow

import (
	"context"
	"reflect"

	"github.com/cschleiden/go-dt/pkg/core"
)

type Executor interface {
}

type executorImpl struct {
}

func (e *executorImpl) ExecuteWorkflow(ctx context.Context, wf Workflow) {
	wfCtx := NewContext()

	// TODO: validate
	t := reflect.TypeOf(wf)
	if t.Kind() != reflect.Func {
		panic("workflow needs to be a function")
	}

	wfv := reflect.ValueOf(wf)
	wfv.Call([]reflect.Value{reflect.ValueOf(wfCtx)})
}

func ExecuteActivity(ctx Context, activity Activity) (core.Future, error) {
	// TODO: Get name of activity
	// TODO: Lookup activity result
	// TODO: Provide result if given
	// TODO: Schedule activity task otherwise

	f := core.NewFuture()
	return f, nil
}
