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
	ExecuteWorkflowTask(ctx context.Context) ([]command.Command, error)
}

type executor struct {
	registry *Registry
	task     *task.Workflow
	workflow *workflow
}

func NewExecutor(registry *Registry, task *task.Workflow) WorkflowExecutor {
	// TODO: Is taking this from the first message the best approach? Should we move
	// this into the task itself? Will this be an issue once we get to sub/child workflows
	// ?
	attributes, ok := task.History[0].Attributes.(history.ExecutionStartedAttributes)
	if !ok {
		panic("workflow task did not contain execution started as first message")
	}

	// TODO: Move this to registry
	// TODO: Support version
	wfFn := registry.GetWorkflow(attributes.Name)
	workflow := NewWorkflow(reflect.ValueOf(wfFn))

	return &executor{
		registry: registry,
		task:     task,
		workflow: workflow,
	}
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context) ([]command.Command, error) {
	// Replay history
	for i := range e.task.History {
		event := &e.task.History[i]

		// TODO: Move the context to be owned by the executor?
		e.workflow.context.SetReplaying(event.Played)

		e.executeEvent(ctx, *event)

		event.Played = true
	}

	// Check if workflow finished
	if e.workflow.Completed() {
		e.workflowCompleted()
	}

	if e.workflow != nil {
		// End workflow if running to prevent leaking goroutines
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
	e.workflow.Execute(ctx, attributes.Inputs) // TODO: handle error
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
