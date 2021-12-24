package workflow

import (
	"context"
	"log"
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
	registry      *Registry
	task          *task.Workflow
	workflow      *workflow
	workflowState *workflowState
}

func NewExecutor(registry *Registry, task *task.Workflow) WorkflowExecutor {
	// TODO: Is taking this from the first message the best approach? Should we move
	// this into the task itself? Will this be an issue once we get to sub/child workflows
	// ?
	var name string

	if len(task.History) == 0 {
		attributes, ok := task.NewEvents[0].Attributes.(history.ExecutionStartedAttributes)
		if !ok {
			panic("workflow task did not contain execution started as first message")
		}
		name = attributes.Name
	} else {
		attributes, ok := task.History[0].Attributes.(history.ExecutionStartedAttributes)
		if !ok {
			panic("workflow task did not contain execution started as first message")
		}
		name = attributes.Name
	}

	// TODO: Move this to registry
	// TODO: Support version
	wfFn := registry.GetWorkflow(name)
	workflow := NewWorkflow(reflect.ValueOf(wfFn))

	return &executor{
		registry:      registry,
		task:          task,
		workflow:      workflow,
		workflowState: newWorkflowState(),
	}
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context) ([]command.Command, error) {
	ctx = withWfState(ctx, e.workflowState)

	// Replay history
	e.workflowState.setReplaying(true)
	for _, event := range e.task.History {
		e.executeEvent(ctx, event)
	}

	// Play new events
	e.workflowState.setReplaying(false)
	for _, event := range e.task.NewEvents {
		e.executeEvent(ctx, event)
	}

	// Check if workflow finished
	if e.workflow.Completed() {
		e.workflowCompleted(ctx, e.workflow.result)
	}

	if e.workflow != nil {
		// End workflow if running to prevent leaking goroutines
		e.workflow.Close(ctx)
	}

	return e.workflowState.commands, nil
}

func (e *executor) executeEvent(ctx context.Context, event history.HistoryEvent) error {
	log.Println("Handling:", event.EventType)

	switch event.EventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		a := event.Attributes.(history.ExecutionStartedAttributes)
		e.handleWorkflowExecutionStarted(ctx, &a)

	case history.HistoryEventType_WorkflowExecutionFinished:

	case history.HistoryEventType_ActivityScheduled:
		a := event.Attributes.(history.ActivityScheduledAttributes)
		e.handleActivityScheduled(ctx, event, &a)

	case history.HistoryEventType_ActivityFailed:

	case history.HistoryEventType_ActivityCompleted:
		a := event.Attributes.(history.ActivityCompletedAttributes)
		e.handleActivityCompleted(ctx, event, &a)

	case history.HistoryEventType_TimerScheduled:
		a := event.Attributes.(history.TimerScheduledAttributes)
		e.handleTimerScheduled(ctx, event, &a)

	case history.HistoryEventType_TimerFired:
		a := event.Attributes.(history.TimerFiredAttributes)
		e.handleTimerFired(ctx, event, &a)

	case history.HistoryEventType_SignalReceived:
		a := event.Attributes.(history.SignalReceivedAttributes)
		e.handleSignalReceived(ctx, event, &a)

	default:
		panic("unknown event type")
	}

	return nil
}

func (e *executor) handleWorkflowExecutionStarted(ctx context.Context, attributes *history.ExecutionStartedAttributes) {
	e.workflow.Execute(ctx, attributes.Inputs) // TODO: handle error
}

func (e *executor) handleActivityScheduled(_ context.Context, event history.HistoryEvent, attributes *history.ActivityScheduledAttributes) {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Ensure the same activity is scheduled again
			ca := c.Attr.(command.ScheduleActivityTaskCommandAttr)
			if attributes.Name != ca.Name {
				panic("Previous workflow execution scheduled different type of activity") // TODO: Return to caller?
			}

			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}
}

func (e *executor) handleActivityCompleted(ctx context.Context, event history.HistoryEvent, a *history.ActivityCompletedAttributes) {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		panic("no pending future!")
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	f.Set(ctx, func(v interface{}) error {
		return converter.DefaultConverter.From(a.Result, v)
	})
	e.workflow.Continue(ctx)
}

func (e *executor) handleTimerScheduled(_ context.Context, event history.HistoryEvent, attributes *history.TimerScheduledAttributes) {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}
}

func (e *executor) handleTimerFired(ctx context.Context, event history.HistoryEvent, attributes *history.TimerFiredAttributes) {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		panic("no pending future!")
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	f.Set(ctx, func(v interface{}) error {
		r := reflect.ValueOf(v)
		r.Elem().Set(reflect.ValueOf(true)) // TODO: Set different value for timer future?

		return nil
	})
	e.workflow.Continue(ctx)
}

func (e *executor) handleSignalReceived(ctx context.Context, event history.HistoryEvent, attributes *history.SignalReceivedAttributes) {

	e.workflow.Continue(ctx)

	// f, ok := e.workflow.Context().pendingFutures[event.EventID]
	// if !ok {
	// 	panic("no pending future!")
	// }

	// // Remove pending command
	// for i, c := range e.workflow.Context().commands {
	// 	if c.ID == event.EventID {
	// 		e.workflow.context.commands = append(e.workflow.context.commands[:i], e.workflow.context.commands[i+1:]...)
	// 		break
	// 	}
	// }

	// f.Set(func(v interface{}) error {
	// 	r := reflect.ValueOf(v)
	// 	r.Elem().Set(reflect.ValueOf(true)) // TODO: Set different value for timer future?

	// 	return nil
	// })
	// e.workflow.Continue(ctx)
}

func (e *executor) workflowCompleted(ctx context.Context, result []byte) {
	eventId := e.workflowState.eventID
	e.workflowState.eventID++

	e.workflowState.addCommand(command.NewCompleteWorkflowCommand(eventId, result))
}
