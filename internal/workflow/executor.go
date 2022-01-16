package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	errs "github.com/pkg/errors"
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

	wfFn := registry.GetWorkflow(name)
	workflow := NewWorkflow(reflect.ValueOf(wfFn))

	return &executor{
		registry:      registry,
		task:          task,
		workflow:      workflow,
		workflowState: newWorkflowState(),
		logger:        log.New(io.Discard, "", log.LstdFlags),
		//logger: log.Default(),
	}
}

func (e *executor) ExecuteWorkflowTask(ctx context.Context) ([]command.Command, error) {
	wfCtx := withWfState(sync.Background(), e.workflowState)

	defer func() {
		if e.workflow != nil {
			// End workflow if running to prevent leaking goroutines
			e.workflow.Close(wfCtx)
		}
	}()

	// Replay history
	e.workflowState.setReplaying(true)
	for _, event := range e.task.History {
		if err := e.executeEvent(wfCtx, event); err != nil {
			return nil, errs.Wrap(err, "error while replaying event")
		}
	}

	// Play new events
	e.workflowState.setReplaying(false)
	for _, event := range e.task.NewEvents {
		if err := e.executeEvent(wfCtx, event); err != nil {
			return nil, errs.Wrap(err, "error while executing event")
		}
	}

	if e.workflow.Completed() {
		if err := e.workflowCompleted(ctx, e.workflow.Result(), e.workflow.Error()); err != nil {
			return nil, err
		}
	}

	return e.workflowState.commands, nil
}

func (e *executor) executeEvent(ctx sync.Context, event history.Event) error {
	e.logger.Println("Handling:", event.Type)

	var err error

	switch event.Type {
	case history.EventType_WorkflowExecutionStarted:
		err = e.handleWorkflowExecutionStarted(ctx, event.Attributes.(*history.ExecutionStartedAttributes))

	case history.EventType_WorkflowExecutionFinished:

	case history.EventType_ActivityScheduled:
		err = e.handleActivityScheduled(ctx, event, event.Attributes.(*history.ActivityScheduledAttributes))

	case history.EventType_ActivityFailed:
		err = e.handleActivityFailed(ctx, event, event.Attributes.(*history.ActivityFailedAttributes))

	case history.EventType_ActivityCompleted:
		err = e.handleActivityCompleted(ctx, event, event.Attributes.(*history.ActivityCompletedAttributes))

	case history.EventType_TimerScheduled:
		err = e.handleTimerScheduled(ctx, event, event.Attributes.(*history.TimerScheduledAttributes))

	case history.EventType_TimerFired:
		err = e.handleTimerFired(ctx, event, event.Attributes.(*history.TimerFiredAttributes))

	case history.EventType_SignalReceived:
		err = e.handleSignalReceived(ctx, event, event.Attributes.(*history.SignalReceivedAttributes))

	case history.EventType_SubWorkflowScheduled:
		err = e.handleSubWorkflowScheduled(ctx, event, event.Attributes.(*history.SubWorkflowScheduledAttributes))

	case history.EventType_SubWorkflowCompleted:
		err = e.handleSubWorkflowCompleted(ctx, event, event.Attributes.(*history.SubWorkflowCompletedAttributes))

	default:
		return fmt.Errorf("unknown event type: %v", event.Type)
	}

	return err
}

func (e *executor) handleWorkflowExecutionStarted(ctx sync.Context, a *history.ExecutionStartedAttributes) error {
	return e.workflow.Execute(ctx, a.Inputs)
}

func (e *executor) handleActivityScheduled(ctx sync.Context, event history.Event, a *history.ActivityScheduledAttributes) error {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Ensure the same activity is scheduled again
			ca := c.Attr.(*command.ScheduleActivityTaskCommandAttr)
			if a.Name != ca.Name {
				return fmt.Errorf("previous workflow execution scheduled different type of activity: %s, %s", a.Name, ca.Name)
			}

			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	return nil
}

func (e *executor) handleActivityCompleted(ctx sync.Context, event history.Event, a *history.ActivityCompletedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		return errors.New("no pending future found for activity completed event")
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	f.Set(a.Result, nil)

	return e.workflow.Continue(ctx)
}

func (e *executor) handleActivityFailed(ctx sync.Context, event history.Event, a *history.ActivityFailedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		return errors.New("no pending future found for activity failed event")
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	f.Set(nil, errors.New(a.Reason))

	return e.workflow.Continue(ctx)
}

func (e *executor) handleTimerScheduled(ctx sync.Context, event history.Event, a *history.TimerScheduledAttributes) error {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	return nil
}

func (e *executor) handleTimerFired(ctx sync.Context, event history.Event, a *history.TimerFiredAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		// Timer already cancelled ignore
		return nil
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	f.Set(nil, nil)

	return e.workflow.Continue(ctx)
}

func (e *executor) handleSubWorkflowScheduled(ctx sync.Context, event history.Event, a *history.SubWorkflowScheduledAttributes) error {
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			// Ensure the same activity is scheduled again
			ca := c.Attr.(*command.ScheduleSubWorkflowCommandAttr)
			if a.Name != ca.Name {
				return errors.New("previous workflow execution scheduled a different sub workflow")
			}

			// Remove pending command
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	return nil
}

func (e *executor) handleSubWorkflowCompleted(ctx sync.Context, event history.Event, a *history.SubWorkflowCompletedAttributes) error {
	f, ok := e.workflowState.pendingFutures[event.EventID]
	if !ok {
		return errors.New("no pending future found for sub workflow completed event")
	}

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	if a.Error != "" {
		f.Set(nil, errors.New(a.Error))
	} else {
		f.Set(a.Result, nil)
	}

	return e.workflow.Continue(ctx)
}

func (e *executor) handleSignalReceived(ctx sync.Context, event history.Event, a *history.SignalReceivedAttributes) error {
	sc := e.workflowState.getSignalChannel(a.Name)

	sc.Send(ctx, a.Arg)

	// Remove pending command
	for i, c := range e.workflowState.commands {
		if c.ID == event.EventID {
			e.workflowState.commands = append(e.workflowState.commands[:i], e.workflowState.commands[i+1:]...)
			break
		}
	}

	return e.workflow.Continue(ctx)
}

func (e *executor) workflowCompleted(ctx context.Context, result payload.Payload, err error) error {
	eventId := e.workflowState.eventID
	e.workflowState.eventID++

	e.workflowState.addCommand(command.NewCompleteWorkflowCommand(eventId, result, err))

	return nil
}
