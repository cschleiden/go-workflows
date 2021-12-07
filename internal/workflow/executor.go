package workflow

import (
	"context"

	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
)

type WorkflowExecutor interface {
	ExecuteWorkflowTask(ctx context.Context, task tasks.WorkflowTask) error
}

func NewExecutor(registry *Registry) WorkflowExecutor {
	return &executor{
		registry: registry,
	}
}

type executor struct {
	registry *Registry
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context, task tasks.WorkflowTask) error {
	wfCtx := NewWorkflowContext()

	// Replay history
	for _, event := range task.History {
		e.executeEvent(ctx, wfCtx, event)

		event.Played = true
	}

	// Check if workflow is completed

	// ----

	// name := "TODO!"

	// wf := e.registry.getWorkflow(name)

	// wfCtx := workflow.Context

	// coState := newCoroutine(ctx, func(ctx context.Context) {
	// 	// Get inputs

	// 	// Execute workflow
	// 	wfFn, ok := wf.(func(ctx workflow.Context, args ...[]interface{}))
	// 	if !ok {
	// 		panic("workflow is not a function")
	// 	}

	// 	wfFn(wfCtx)
	// })

	// <-coState.blocking

	// if coState.finished.Load().(bool) {
	// 	// Workflow execution done, end?
	// }

	// // 1. Replay history
	// // 2. Start new coroutine
	// // 3. Wait for completion

	return nil
}

func (e *executor) executeEvent(ctx context.Context, wfCtx Context, event history.HistoryEvent) error {
	switch event.EventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		a := event.Attributes.(*history.ExecutionStartedAttributes)
		e.executeWorkflow(wfCtx, a)
	default:
		panic("unknown event type")
	}

	return nil
}

func (e *executor) executeWorkflow(wfCtx Context, attributes *history.ExecutionStartedAttributes) {
	// coState := newCoroutine(ctx, func(ctx context.Context) {
	// })
}

// func (e *executorImpl) ExecuteWorkflow(ctx context.Context, wf Workflow) {
// 	wfCtx := NewContext()

// 	// TODO: validate
// 	t := reflect.TypeOf(wf)
// 	if t.Kind() != reflect.Func {
// 		panic("workflow needs to be a function")
// 	}

// 	wfv := reflect.ValueOf(wf)
// 	wfv.Call([]reflect.Value{reflect.ValueOf(wfCtx)})
// }

// func ExecuteActivity(ctx Context, activity Activity) (core.Future, error) {
// 	// TODO: Get name of activity
// 	// TODO: Lookup activity result
// 	// TODO: Provide result if given
// 	// TODO: Schedule activity task otherwise

// 	f := core.NewFuture()
// 	return f, nil
// }
