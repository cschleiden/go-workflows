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
	registry *Registry
	workflow *workflow
}

func NewExecutor(registry *Registry) WorkflowExecutor {
	return &executor{
		registry: registry,
	}
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context, task tasks.WorkflowTask) error {
	// Replay history
	for _, event := range task.History {
		e.executeEvent(ctx, event)

		event.Played = true
	}

	// Check if workflow finished

	// TODO: Process commands

	if e.workflow != nil {
		e.workflow.Close()
	}
	return nil
}

func (e *executor) executeEvent(ctx context.Context, event history.HistoryEvent) error {
	switch event.EventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		a := event.Attributes.(*history.ExecutionStartedAttributes)
		e.executeWorkflow(ctx, a)

	case history.HistoryEventType_ActivityScheduled:
		e.handleActivityScheduled(ctx)

	case history.HistoryEventType_ActivityFailed:

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
	e.workflow = NewWorkflow(wfFn)

	e.workflow.Execute(ctx) // TODO: handle error
}

func (e *executor) handleActivityScheduled(_ context.Context) {
}

func (e *executor) handleActivityCompleted(ctx context.Context, attributes *history.ActivityCompletedAttributes) {
	f, ok := e.workflow.Context().openFutures[attributes.ScheduleID] // TODO: not quite the right id
	if !ok {
		panic("no future!")
	}

	f.Set(attributes.Result) // TODO: Deserialize

	e.workflow.Continue(ctx)
}
