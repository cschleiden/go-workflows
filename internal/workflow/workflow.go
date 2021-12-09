package workflow

import (
	"context"
	"reflect"

	"github.com/cschleiden/go-dt/internal/sync"
)

type Workflow interface{}

type workflowFn func(Context) error // TODO: args

type workflow struct {
	context *contextImpl
	cr      sync.Coroutine
	fn      reflect.Value
}

func NewWorkflow(workflowFn reflect.Value) *workflow {
	c := sync.NewCoroutine()

	return &workflow{
		context: newWorkflowContext(c),
		cr:      c,
		fn:      workflowFn,
	}
}

func (w *workflow) Context() *contextImpl {
	return w.context
}

func (w *workflow) Execute(ctx context.Context) error {
	w.cr.Run(ctx, func(ctx context.Context) {
		// TODO: Support inputs
		w.fn.Call([]reflect.Value{reflect.ValueOf(w.context)})
	})

	w.cr.WaitUntilBlocked()

	return nil
}

func (w *workflow) Continue(ctx context.Context) error {
	w.cr.Continue()

	return nil
}

func (w *workflow) Completed() bool {
	return w.cr.Finished()
}

func (w *workflow) Close() {
	// End coroutine execution to prevent goroutine leaks
	w.cr.Exit()
}
