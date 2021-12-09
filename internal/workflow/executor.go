package workflow

import (
	"context"
	"reflect"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/converter"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
)

type WorkflowExecutor interface {
	ExecuteWorkflowTask(ctx context.Context, task task.Workflow) ([]command.Command, error)
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

func (e *executor) ExecuteWorkflowTask(ctx context.Context, task task.Workflow) ([]command.Command, error) {
	// Replay history
	for _, event := range task.History {
		e.executeEvent(ctx, event)

		event.Played = true
	}

	// Check if workflow finished
	if e.workflow.Completed() {
		e.workflowCompleted()
	}

	if e.workflow != nil {
		// End workflow when running to prevent leaking goroutines
		e.workflow.Close()
	}

	return e.workflow.context.commands, nil
}

func (e *executor) executeEvent(ctx context.Context, event history.HistoryEvent) error {
	switch event.EventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		a := event.Attributes.(history.ExecutionStartedAttributes)
		e.handleWorkflowExecutionStarted(ctx, &a)

	case history.HistoryEventType_ActivityScheduled:
		e.handleActivityScheduled(ctx)

	case history.HistoryEventType_ActivityFailed:

	case history.HistoryEventType_ActivityCompleted:
		a := event.Attributes.(history.ActivityCompletedAttributes)
		e.handleActivityCompleted(ctx, event, &a)

	default:
		panic("unknown event type")
	}

	return nil
}

func (e *executor) handleWorkflowExecutionStarted(ctx context.Context, attributes *history.ExecutionStartedAttributes) {
	// TODO: Move this to registry
	wf := e.registry.GetWorkflow(attributes.Name)
	e.workflow = NewWorkflow(reflect.ValueOf(wf))

	e.workflow.Execute(ctx) // TODO: handle error
}

func (e *executor) handleActivityScheduled(_ context.Context) {
}

func (e *executor) handleActivityCompleted(ctx context.Context, event history.HistoryEvent, a *history.ActivityCompletedAttributes) {
	f, ok := e.workflow.Context().pendingFutures[event.EventID]
	if !ok {
		panic("no pending future!")
	}

	// Remove pending command
	for i, c := range e.workflow.Context().commands {
		if c.ID == event.EventID {
			e.workflow.context.commands = append(e.workflow.context.commands[:i], e.workflow.context.commands[i+1:]...)
			break
		}
	}

	f.Set(func(v interface{}) error {
		return converter.DefaultConverter.From(a.Result, v)
	})
	e.workflow.Continue(ctx)
}

func (e *executor) workflowCompleted() {
	wfCtx := e.workflow.Context()

	eventId := wfCtx.eventID
	wfCtx.eventID++

	wfCtx.AddCommand(command.NewCompleteWorkflowCommand(eventId))
}
