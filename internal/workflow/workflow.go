package workflow

import (
	"context"

	"github.com/cschleiden/go-dt/internal/sync"
)

type Workflow interface{}

type workflowFn func(Context) error // TODO: args

type workflow struct {
	context *contextImpl
	cr      sync.Coroutine
	fn      workflowFn
}

func NewWorkflow(workflowFn workflowFn) *workflow {
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
		w.fn(w.context)
	})

	w.cr.WaitUntilBlocked()

	return nil
}

func (w *workflow) Continue(ctx context.Context) error {
	w.cr.Continue()

	return nil
}

func (w *workflow) Complete() bool {
	return w.cr.Finished()
}
