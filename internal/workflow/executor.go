package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/core/task"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/google/uuid"
	errs "github.com/pkg/errors"
)

type WorkflowExecutor interface {
	ExecuteTask(ctx context.Context, t *task.Workflow) ([]history.Event, []history.WorkflowEvent, error)

	Close()
}

type executor struct {
	registry          *Registry
	workflow          *workflow
	workflowState     *workflowState
	workflowCtx       sync.Context
	workflowCtxCancel sync.CancelFunc
	clock             clock.Clock
	logger            *log.Logger
	lastEventID       string // TODO: Not the same as the sequence number Event ID
}

func NewExecutor(registry *Registry, instance core.WorkflowInstance, clock clock.Clock) (WorkflowExecutor, error) {
	state := newWorkflowState(instance, clock)
	wfCtx, cancel := sync.WithCancel(withWfState(sync.Background(), state))

	return &executor{
		registry:          registry,
		workflowState:     state,
		workflowCtx:       wfCtx,
		workflowCtxCancel: cancel,
		clock:             clock,
		logger:            log.New(io.Discard, "", log.LstdFlags),
		//logger: log.Default(),
	}, nil
}

func (e *executor) ExecuteTask(ctx context.Context, t *task.Workflow) ([]history.Event, []history.WorkflowEvent, error) {
	if t.Kind == task.Continuation {
		// Check if the current state matches the backend's history state
		newestHistoryEvent := t.History[len(t.History)-1]
		if newestHistoryEvent.ID != e.lastEventID {
			return nil, nil, fmt.Errorf("mismatch in execution, last event %v not found in history, last there is %v (%v)", e.lastEventID, newestHistoryEvent.ID, newestHistoryEvent.Type)
		}

		// Clear commands from previous executions
		e.workflowState.clearCommands()
	} else {
		// Replay history
		e.workflowState.setReplaying(true)
		for _, event := range t.History {
			if err := e.executeEvent(event); err != nil {
				return nil, nil, errs.Wrap(err, "error while replaying event")
			}
		}
	}

	// Always pad the received events with WorkflowTaskStarted/Finished events to indicate the execution
	events := []history.Event{history.NewHistoryEvent(e.clock.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})}
	events = append(events, t.NewEvents...)

	// Execute new events received from the backend
	if err := e.executeNewEvents(events); err != nil {
		return nil, nil, errs.Wrap(err, "error while executing new events")
	}

	newCommandEvents, workflowEvents, err := e.processCommands(ctx, t)
	if err != nil {
		return nil, nil, errs.Wrap(err, "could not process commands")
	}
	events = append(events, newCommandEvents...)

	// Execution of this task is finished, add event to history
	events = append(events, history.NewHistoryEvent(e.clock.Now(), history.EventType_WorkflowTaskFinished, &history.WorkflowTaskFinishedAttributes{}))

	e.lastEventID = events[len(events)-1].ID

	return events, workflowEvents, nil
}

func (e *executor) executeNewEvents(newEvents []history.Event) error {
	e.workflowState.setReplaying(false)

	for _, event := range newEvents {
		if err := e.executeEvent(event); err != nil {
			return errs.Wrap(err, "error while executing event")
		}

		// Remember that we executed this event last
		e.lastEventID = event.ID
	}

	if e.workflow.Completed() {
		if err := e.workflowCompleted(e.workflow.Result(), e.workflow.Error()); err != nil {
			return err
		}
	}

	return nil
}

func (e *executor) Close() {
	if e.workflow != nil {
		// End workflow if running to prevent leaking goroutines
		e.workflow.Close(e.workflowCtx)
	}
}

func (e *executor) executeEvent(event history.Event) error {
	e.logger.Println("Handling:", event.Type)

	var err error

	switch event.Type {
	case history.EventType_WorkflowExecutionStarted:
		err = e.handleWorkflowExecutionStarted(event.Attributes.(*history.ExecutionStartedAttributes))

	case history.EventType_WorkflowExecutionFinished:
	// Ignore

	case history.EventType_WorkflowExecutionCanceled:
		err = e.handleWorkflowCanceled()

	case history.EventType_WorkflowTaskStarted:
		err = e.handleWorkflowTaskStarted(event, event.Attributes.(*history.WorkflowTaskStartedAttributes))

	case history.EventType_WorkflowTaskFinished:
		// Ignore

	case history.EventType_ActivityScheduled:
		err = e.handleActivityScheduled(event, event.Attributes.(*history.ActivityScheduledAttributes))

	case history.EventType_ActivityFailed:
		err = e.handleActivityFailed(event, event.Attributes.(*history.ActivityFailedAttributes))

	case history.EventType_ActivityCompleted:
		err = e.handleActivityCompleted(event, event.Attributes.(*history.ActivityCompletedAttributes))

	case history.EventType_TimerScheduled:
		err = e.handleTimerScheduled(event, event.Attributes.(*history.TimerScheduledAttributes))

	case history.EventType_TimerFired:
		err = e.handleTimerFired(event, event.Attributes.(*history.TimerFiredAttributes))

	case history.EventType_SignalReceived:
		err = e.handleSignalReceived(event, event.Attributes.(*history.SignalReceivedAttributes))

	case history.EventType_SideEffectResult:
		err = e.handleSideEffectResult(event, event.Attributes.(*history.SideEffectResultAttributes))

	case history.EventType_SubWorkflowScheduled:
		err = e.handleSubWorkflowScheduled(event, event.Attributes.(*history.SubWorkflowScheduledAttributes))

	case history.EventType_SubWorkflowFailed:
		err = e.handleSubWorkflowFailed(event, event.Attributes.(*history.SubWorkflowFailedAttributes))

	case history.EventType_SubWorkflowCompleted:
		err = e.handleSubWorkflowCompleted(event, event.Attributes.(*history.SubWorkflowCompletedAttributes))

	default:
		return fmt.Errorf("unknown event type: %v", event.Type)
	}

	return err
}

func (e *executor) handleWorkflowExecutionStarted(a *history.ExecutionStartedAttributes) error {
	wfFn, err := e.registry.GetWorkflow(a.Name)
	if err != nil {
		return fmt.Errorf("workflow %s not found", a.Name)
	}

	e.workflow = NewWorkflow(reflect.ValueOf(wfFn))

	return e.workflow.Execute(e.workflowCtx, a.Inputs)
}

func (e *executor) handleWorkflowCanceled() error {
	e.workflowCtxCancel()

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleWorkflowTaskStarted(event history.Event, a *history.WorkflowTaskStartedAttributes) error {
	e.workflowState.setTime(event.Timestamp)

	return nil
}

func (e *executor) handleActivityScheduled(event history.Event, a *history.ActivityScheduledAttributes) error {
	c := e.workflowState.removeCommandByEventID(event.ScheduleEventID)
	if c != nil {
		// Ensure the same activity is scheduled again
		ca := c.Attr.(*command.ScheduleActivityTaskCommandAttr)
		if a.Name != ca.Name {
			return fmt.Errorf("previous workflow execution scheduled different type of activity: %s, %s", a.Name, ca.Name)
		}
	}

	return nil
}

func (e *executor) handleActivityCompleted(event history.Event, a *history.ActivityCompletedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.ScheduleEventID]
	if !ok {
		return nil
	}

	e.workflowState.removeCommandByEventID(event.ScheduleEventID)
	f.Set(a.Result, nil)

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleActivityFailed(event history.Event, a *history.ActivityFailedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.ScheduleEventID]
	if !ok {
		return errors.New("no pending future found for activity failed event")
	}

	e.workflowState.removeCommandByEventID(event.ScheduleEventID)

	f.Set(nil, errors.New(a.Reason))

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleTimerScheduled(event history.Event, a *history.TimerScheduledAttributes) error {
	e.workflowState.removeCommandByEventID(event.ScheduleEventID)

	return nil
}

func (e *executor) handleTimerFired(event history.Event, a *history.TimerFiredAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.ScheduleEventID]
	if !ok {
		// Timer already canceled ignore
		return nil
	}

	e.workflowState.removeCommandByEventID(event.ScheduleEventID)

	f.Set(nil, nil)

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleSubWorkflowScheduled(event history.Event, a *history.SubWorkflowScheduledAttributes) error {
	c := e.workflowState.removeCommandByEventID(event.ScheduleEventID)
	if c != nil {
		ca := c.Attr.(*command.ScheduleSubWorkflowCommandAttr)
		if a.Name != ca.Name {
			return errors.New("previous workflow execution scheduled a different sub workflow")
		}
	}

	return nil
}

func (e *executor) handleSubWorkflowFailed(event history.Event, a *history.SubWorkflowFailedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.ScheduleEventID]
	if !ok {
		return errors.New("no pending future found for sub workflow failed event")
	}

	e.workflowState.removeCommandByEventID(event.ScheduleEventID)

	f.Set(nil, errors.New(a.Error))

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleSubWorkflowCompleted(event history.Event, a *history.SubWorkflowCompletedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.ScheduleEventID]
	if !ok {
		return errors.New("no pending future found for sub workflow completed event")
	}

	e.workflowState.removeCommandByEventID(event.ScheduleEventID)

	f.Set(a.Result, nil)

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleSignalReceived(event history.Event, a *history.SignalReceivedAttributes) error {
	// Send signal to workflow channel
	sc := e.workflowState.getSignalChannel(a.Name)
	sc.SendNonblocking(e.workflowCtx, a.Arg)

	e.workflowState.removeCommandByEventID(event.ScheduleEventID)

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) handleSideEffectResult(event history.Event, a *history.SideEffectResultAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.ScheduleEventID]
	if !ok {
		return errors.New("no pending future found for side effect result event")
	}

	f.Set(a.Result, nil)

	return e.workflow.Continue(e.workflowCtx)
}

func (e *executor) workflowCompleted(result payload.Payload, err error) error {
	eventId := e.workflowState.scheduleEventID
	e.workflowState.scheduleEventID++

	cmd := command.NewCompleteWorkflowCommand(eventId, result, err)
	e.workflowState.addCommand(&cmd)

	return nil
}

func (e *executor) processCommands(ctx context.Context, t *task.Workflow) ([]history.Event, []history.WorkflowEvent, error) {
	instance := t.WorkflowInstance
	commands := e.workflowState.commands

	newEvents := make([]history.Event, 0)
	workflowEvents := make([]history.WorkflowEvent, 0)

	for _, c := range commands {
		// TODO: Move to state machine?
		// Mark this command as committed.
		c.State = command.CommandState_Committed

		switch c.Type {
		case command.CommandType_ScheduleActivityTask:
			a := c.Attr.(*command.ScheduleActivityTaskCommandAttr)

			newEvents = append(newEvents, history.NewHistoryEvent(
				e.clock.Now(),
				history.EventType_ActivityScheduled,
				&history.ActivityScheduledAttributes{
					Name:   a.Name,
					Inputs: a.Inputs,
				},
				history.ScheduleEventID(c.ID),
			))

		case command.CommandType_ScheduleSubWorkflow:
			a := c.Attr.(*command.ScheduleSubWorkflowCommandAttr)

			subWorkflowInstance := core.NewSubWorkflowInstance(a.InstanceID, uuid.NewString(), instance, c.ID)

			newEvents = append(newEvents, history.NewHistoryEvent(
				e.clock.Now(),
				history.EventType_SubWorkflowScheduled,
				&history.SubWorkflowScheduledAttributes{
					InstanceID: subWorkflowInstance.GetInstanceID(),
					Name:       a.Name,
					Inputs:     a.Inputs,
				},
				history.ScheduleEventID(c.ID),
			))

			// Send message to new workflow instance
			workflowEvents = append(workflowEvents, history.WorkflowEvent{
				WorkflowInstance: subWorkflowInstance,
				HistoryEvent: history.NewHistoryEvent(
					e.clock.Now(),
					history.EventType_WorkflowExecutionStarted,
					&history.ExecutionStartedAttributes{
						Name:   a.Name,
						Inputs: a.Inputs,
					},
					history.ScheduleEventID(c.ID),
				),
			})

		case command.CommandType_SideEffect:
			a := c.Attr.(*command.SideEffectCommandAttr)
			newEvents = append(newEvents, history.NewHistoryEvent(
				e.clock.Now(),
				history.EventType_SideEffectResult,
				&history.SideEffectResultAttributes{
					Result: a.Result,
				},
				history.ScheduleEventID(c.ID),
			))

		case command.CommandType_ScheduleTimer:
			a := c.Attr.(*command.ScheduleTimerCommandAttr)

			newEvents = append(newEvents, history.NewHistoryEvent(
				e.clock.Now(),
				history.EventType_TimerScheduled,
				&history.TimerScheduledAttributes{
					At: a.At,
				},
				history.ScheduleEventID(c.ID),
			))

			// Create timer_fired event which will become visible in the future
			workflowEvents = append(workflowEvents, history.WorkflowEvent{
				WorkflowInstance: instance,
				HistoryEvent: history.NewHistoryEvent(
					e.clock.Now(),
					history.EventType_TimerFired,
					&history.TimerFiredAttributes{
						At: a.At,
					},
					history.ScheduleEventID(c.ID),
					history.VisibleAt(a.At),
				)},
			)

		case command.CommandType_CompleteWorkflow:
			a := c.Attr.(*command.CompleteWorkflowCommandAttr)

			newEvents = append(newEvents, history.NewHistoryEvent(
				e.clock.Now(),
				history.EventType_WorkflowExecutionFinished,
				&history.ExecutionCompletedAttributes{
					Result: a.Result,
					Error:  a.Error,
				},
				history.ScheduleEventID(c.ID),
			))

			if instance.SubWorkflow() {
				// Send completion message back to parent workflow instance
				var historyEvent history.Event

				if a.Error != "" {
					// Sub workflow failed
					historyEvent = history.NewHistoryEvent(
						e.clock.Now(),
						history.EventType_SubWorkflowFailed,
						&history.SubWorkflowFailedAttributes{
							Error: a.Error,
						},
						// Ensure the message gets sent back to the parent workflow with the right eventID
						history.ScheduleEventID(instance.ParentEventID()),
					)
				} else {
					historyEvent = history.NewHistoryEvent(
						e.clock.Now(),
						history.EventType_SubWorkflowCompleted,
						&history.SubWorkflowCompletedAttributes{
							Result: a.Result,
						},
						// Ensure the message gets sent back to the parent workflow with the right eventID
						history.ScheduleEventID(instance.ParentEventID()),
					)
				}

				workflowEvents = append(workflowEvents, history.WorkflowEvent{
					WorkflowInstance: instance.ParentInstance(),
					HistoryEvent:     historyEvent,
				})
			}

		default:
			return nil, nil, fmt.Errorf("unknown command type: %v", c.Type)
		}
	}

	return newEvents, workflowEvents, nil
}
