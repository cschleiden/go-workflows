package workflow

import "context"

type Workflow interface{}

type workflowFn func(Context) error // TODO: args

type workflow struct {
	context *contextImpl
	cs      *coState
	fn      workflowFn
}

func NewWorkflow(workflowFn workflowFn) *workflow {
	cs := newState()

	return &workflow{
		context: newWorkflowContext(cs),
		cs:      cs,
		fn:      workflowFn,
	}
}

func (w *workflow) Context() *contextImpl {
	return w.context
}

func (w *workflow) Execute(ctx context.Context) error {
	w.cs.run(ctx, func(ctx context.Context) {
		w.fn(w.context)
	})

	w.cs.UntilBlocked()

	return nil
}

func (w *workflow) Continue(ctx context.Context) error {
	w.cs.cont()

	return nil
}

func (w *workflow) Complete() bool {
	return w.cs.finished.Load().(bool)
}
