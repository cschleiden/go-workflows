package workflow

import (
	"context"
	"io"
	"log"
	"reflect"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
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
	logger        *log.Logger
}

func NewExecutor(registry *Registry, task *task.Workflow) WorkflowExecutor {
	// TODO: Is taking this from the first message the best approach? Should we move
	// this into the task itself? Will this be an issue once we get to sub/child workflows
	// ?
	var name string

	if len(task.History) == 0 {
		a, ok := task.NewEvents[0].Attributes.(*history.ExecutionStartedAttributes)
		if !ok {
			panic("workflow task did not contain execution started as first message")
		}
		name = a.Name
	} else {
		a, ok := task.History[0].Attributes.(*history.ExecutionStartedAttributes)
		if !ok {
			panic("workflow task did not contain execution started as first message")
		}
		name = a.Name
	}

	// TODO: Support version?
	wfFn := registry.GetWorkflow(name)
	workflow := NewWorkflow(reflect.ValueOf(wfFn))

	return &executor{
		registry:      registry,
		task:          task,
		workflow:      workflow,
		workflowState: newWorkflowState(),
		logger:        log.New(io.Discard, "", log.LstdFlags),
	}
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context) ([]command.Command, error) {
	wfCtx := withWfState(sync.Background(), e.workflowState)

	// Replay history
	e.workflowState.setReplaying(true)
	for _, event := range e.task.History {
		e.executeEvent(wfCtx, event)
	}

	// Play new events
	e.workflowState.setReplaying(false)
	for _, event := range e.task.NewEvents {
		e.executeEvent(wfCtx, event)
	}

	// Check if workflow finished
	if e.workflow.Completed() {
		e.workflowCompleted(ctx, e.workflow.result)
	}

	if e.workflow != nil {
		// End workflow if running to prevent leaking goroutines
		e.workflow.Close(wfCtx)
	}

	return e.workflowState.commands, nil
}

func (e *executor) executeEvent(ctx sync.Context, event history.HistoryEvent) error {
	e.logger.Println("Handling:", event.EventType)

	switch event.EventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		e.handleWorkflowExecutionStarted(ctx, event.Attributes.(*history.ExecutionStartedAttributes))

	case history.HistoryEventType_WorkflowExecutionFinished:

	case history.HistoryEventType_ActivityScheduled:
		e.handleActivityScheduled(ctx, event, event.Attributes.(*history.ActivityScheduledAttributes))

	case history.HistoryEventType_ActivityFailed:
		// TODO: Nothing to handle, yet?

	case history.HistoryEventType_ActivityCompleted:
		e.handleActivityCompleted(ctx, event, event.Attributes.(*history.ActivityCompletedAttributes))

	case history.HistoryEventType_TimerScheduled:
		e.handleTimerScheduled(ctx, event, event.Attributes.(*history.TimerScheduledAttributes))

	case history.HistoryEventType_TimerFired:
		e.handleTimerFired(ctx, event, event.Attributes.(*history.TimerFiredAttributes))

	case history.HistoryEventType_SignalReceived:
		e.handleSignalReceived(ctx, event, event.Attributes.(*history.SignalReceivedAttributes))

	case history.HistoryEventType_SubWorkflowScheduled:
		e.handleSubWorkflowScheduled(ctx, event, event.Attributes.(*history.SubWorkflowScheduledAttributes))

	case history.HistoryEventType_SubWorkflowCompleted:
		e.handleSubWorkflowCompleted(ctx, event, event.Attributes.(*history.SubWorkflowCompletedAttributes))

	default:
		panic("unknown event type")
	}

	return nil
}

func (e *executor) handleWorkflowExecutionStarted(ctx sync.Context, a *history.ExecutionStartedAttributes) {
	e.workflow.Execute(ctx, a.Inputs) // TODO: handle error
}

func (e *executor) handleActivityScheduled(ctx sync.Context, event history.HistoryEvent, a *history.ActivityScheduledAttributes) {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Ensure the same activity is scheduled again
			ca := c.Attr.(*command.ScheduleActivityTaskCommandAttr)
			if a.Name != ca.Name {
				panic("Previous workflow execution scheduled different type of activity") // TODO: Return to caller?
			}

			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}
}

func (e *executor) handleActivityCompleted(ctx sync.Context, event history.HistoryEvent, a *history.ActivityCompletedAttributes) {
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

	f.Set(a.Result, nil)

	// TODO: Handle error
	e.workflow.Continue(ctx)
}

func (e *executor) handleTimerScheduled(ctx sync.Context, event history.HistoryEvent, a *history.TimerScheduledAttributes) {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}
}

func (e *executor) handleTimerFired(ctx sync.Context, event history.HistoryEvent, a *history.TimerFiredAttributes) {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		// Timer already cancelled ignore
		return
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	f.Set(nil, nil)

	// TODO: Handle error
	e.workflow.Continue(ctx)
}

func (e *executor) handleSubWorkflowScheduled(ctx sync.Context, event history.HistoryEvent, a *history.SubWorkflowScheduledAttributes) {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Ensure the same activity is scheduled again
			ca := c.Attr.(*command.ScheduleSubWorkflowCommandAttr)
			if a.Name != ca.Name {
				panic("Previous workflow execution scheduled a different sub workflow") // TODO: Return to caller?
			}

			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}
}

func (e *executor) handleSubWorkflowCompleted(ctx sync.Context, event history.HistoryEvent, a *history.SubWorkflowCompletedAttributes) {
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

	f.Set(a.Result, nil)

	// TODO: Handle error
	e.workflow.Continue(ctx)
}

func (e *executor) handleSignalReceived(ctx sync.Context, event history.HistoryEvent, a *history.SignalReceivedAttributes) {
	sc := e.workflowState.getSignalChannel(a.Name)

	sc.Send(ctx, a.Arg)

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	// TODO: Handle error
	e.workflow.Continue(ctx)
}

func (e *executor) workflowCompleted(ctx context.Context, result payload.Payload) {
	eventId := e.workflowState.eventID
	e.workflowState.eventID++

	e.workflowState.addCommand(command.NewCompleteWorkflowCommand(eventId, result))
}
