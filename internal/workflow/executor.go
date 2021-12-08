package workflow

import (
	"context"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/converter"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
)

type WorkflowExecutor interface {
	ExecuteWorkflowTask(ctx context.Context, task tasks.Workflow) ([]command.Command, error)
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

func (e *executor) ExecuteWorkflowTask(ctx context.Context, task tasks.Workflow) ([]command.Command, error) {
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
		e.handleActivityCompleted(ctx, &a)

	default:
		panic("unknown event type")
	}

	return nil
}

func (e *executor) handleWorkflowExecutionStarted(ctx context.Context, attributes *history.ExecutionStartedAttributes) {
	// TODO: Move this to registry
	wf := e.registry.GetWorkflow(attributes.Name)
	wfFn := wf.(func(Context) error)

	e.workflow = NewWorkflow(wfFn)

	e.workflow.Execute(ctx) // TODO: handle error
}

func (e *executor) handleActivityScheduled(_ context.Context) {
}

func (e *executor) handleActivityCompleted(ctx context.Context, attributes *history.ActivityCompletedAttributes) {
	f, ok := e.workflow.Context().pendingFutures[attributes.ScheduleID] // TODO: not quite the right id
	if !ok {
		panic("no pending future!")
	}

	// Remove pending command
	for i, c := range e.workflow.Context().commands {
		if c.ID == attributes.ScheduleID {
			e.workflow.context.commands = append(e.workflow.context.commands[:i], e.workflow.context.commands[i+1:]...)
			break
		}
	}

	var r interface{}
	converter.DefaultConverter.From(attributes.Result, &r) // TODO: handle error
	f.Set(r)

	e.workflow.Continue(ctx)
}

func (e *executor) workflowCompleted() {
	wfCtx := e.workflow.Context()

	wfCtx.eventID++
	eventId := wfCtx.eventID

	wfCtx.AddCommand(command.NewCompleteWorkflowCommand(eventId))
}
