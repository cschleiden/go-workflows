package workflow

import (
	"context"

	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
)

type WorkflowExecutor interface {
	ExecuteWorkflowTask(ctx context.Context, task tasks.WorkflowTask) error
}

type executor struct {
	registry  *Registry
	wfContext *contextImpl
}

func NewExecutor(registry *Registry) WorkflowExecutor {
	return &executor{
		registry:  registry,
		wfContext: newWorkflowContext(),
	}
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context, task tasks.WorkflowTask) error {
	// Replay history
	for _, event := range task.History {
		e.executeEvent(ctx, event)

		event.Played = true
	}

	// TODO: Process commands

	return nil
}

func (e *executor) executeEvent(ctx context.Context, event history.HistoryEvent) error {
	switch event.EventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		a := event.Attributes.(*history.ExecutionStartedAttributes)
		e.executeWorkflow(ctx, a)

	case history.HistoryEventType_ActivityScheduled:

	case history.HistoryEventType_ActivityCompleted:
		a := event.Attributes.(*history.ActivityCompletedAttributes)
		e.handleActivityCompleted(ctx, a)

	default:
		panic("unknown event type")
	}

	return nil
}

func (e *executor) executeWorkflow(ctx context.Context, attributes *history.ExecutionStartedAttributes) {
	wf := e.registry.getWorkflow(attributes.Name)
	wfFn := wf.(func(Context) error)

	cs := newCoroutine(ctx, func(ctx context.Context) {
		cs := getCoState(ctx)

		e.wfContext.cs = cs

		wfFn(e.wfContext)
	})

	// TODO: Is that how we should wait?
	<-cs.blocking
}

func (e *executor) handleActivityCompleted(ctx context.Context, attributes *history.ActivityCompletedAttributes) {
	f, ok := e.wfContext.openFutures[attributes.ScheduleID] // TODO: not quite the right id
	if !ok {
		panic("no future!")
	}

	f.Set(attributes.Result) // TODO: Deserialize

	e.wfContext.cs.cont()
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
