package workflow

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"github.com/cschleiden/go-workflows/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ExecutionResult struct {
	Completed      bool
	Executed       []history.Event
	ActivityEvents []history.Event
	TimerEvents    []history.Event
	WorkflowEvents []history.WorkflowEvent
}

type WorkflowHistoryProvider interface {
	GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]history.Event, error)
}

type WorkflowExecutor interface {
	ExecuteTask(ctx context.Context, t *task.Workflow) (*ExecutionResult, error)

	Close()
}

type executor struct {
	registry           *Registry
	historyProvider    WorkflowHistoryProvider
	workflow           *workflow
	workflowTracer     *workflowtracer.WorkflowTracer
	workflowState      *workflowstate.WfState
	workflowCtx        sync.Context
	workflowCtxCancel  sync.CancelFunc
	clock              clock.Clock
	logger             log.Logger
	tracer             trace.Tracer
	lastSequenceID     int64
	wfStartedEventSeen bool
}

func NewExecutor(logger log.Logger, tracer trace.Tracer, registry *Registry, historyProvider WorkflowHistoryProvider, instance *core.WorkflowInstance, clock clock.Clock) (WorkflowExecutor, error) {
	s := workflowstate.NewWorkflowState(instance, logger, clock)

	wfTracer := workflowtracer.New(tracer)

	wfCtx, cancel := sync.WithCancel(
		workflowstate.WithWorkflowState(
			workflowtracer.WithWorkflowTracer(
				sync.Background(),
				wfTracer,
			),
			s,
		),
	)

	return &executor{
		registry:           registry,
		historyProvider:    historyProvider,
		workflowTracer:     wfTracer,
		workflowState:      s,
		workflowCtx:        wfCtx,
		workflowCtxCancel:  cancel,
		clock:              clock,
		logger:             logger,
		tracer:             tracer,
		wfStartedEventSeen: false,
	}, nil
}

func (e *executor) ExecuteTask(ctx context.Context, t *task.Workflow) (*ExecutionResult, error) {
	ctx = tracing.UnmarshalSpan(ctx, t.Metadata)
	ctx, span := e.tracer.Start(ctx, "WorkflowTaskExecution", trace.WithAttributes(
		attribute.String(tracing.WorkflowInstanceID, t.WorkflowInstance.InstanceID),
		attribute.String(tracing.WorkflowTaskID, t.ID),
		attribute.Int(tracing.WorkflowTaskEvents, len(t.NewEvents)),
	))
	defer span.End()

	// Make the current span available to the tracer that we pass into the workflow execution. With caching
	// the executor instance might be used for multiple workflow tasks and we want calls made in each task
	// execution to be associated with the span for the WorkflowTaskExecution.
	e.workflowTracer.UpdateExecution(span)

	logger := e.logger.With("task_id", t.ID, "instance_id", t.WorkflowInstance.InstanceID)

	logger.Debug("Executing workflow task", "task_last_sequence_id", t.LastSequenceID)

	if t.WorkflowInstanceState == core.WorkflowInstanceStateFinished {
		// This could happen if signals are delivered after the workflow is finished
		logger.Error("Received workflow task for finished workflow instance, discarding events")

		// Log events that caused this task to be scheduled
		for _, event := range t.NewEvents {
			logger.Debug("Discarded event:", "id", event.ID, "event_type", event.Type.String(), "schedule_event_id", event.ScheduleEventID)
		}

		return &ExecutionResult{
			Completed: true,
		}, nil
	}

	skipNewEvents := false

	if t.LastSequenceID > e.lastSequenceID {
		logger.Debug("Task has newer history than current state, fetching and replaying history", "task_sequence_id", t.LastSequenceID, "local_sequence_id", e.lastSequenceID)

		h, err := e.historyProvider.GetWorkflowInstanceHistory(ctx, t.WorkflowInstance, &e.lastSequenceID)
		if err != nil {
			return nil, fmt.Errorf("getting workflow history: %w", err)
		}

		if err := e.replayHistory(h); err != nil {
			logger.Error("Error while replaying history", "error", err)

			// Fail workflow with an error. Skip executing new events, but still go through the commands
			e.workflowCompleted(nil, err)
			skipNewEvents = true

			// With an error occurred during replay, we need to ensure new events don't get duplicate sequence ids
			e.lastSequenceID = t.LastSequenceID
		} else if t.LastSequenceID != e.lastSequenceID {
			logger.Error("After replaying history, task still has newer history than current state", "task_sequence_id", t.LastSequenceID, "local_sequence_id", e.lastSequenceID)

			return nil, errors.New("even after fetching history and replaying history executor state does not match task")
		}
	} else if t.LastSequenceID < e.lastSequenceID {
		return nil, fmt.Errorf("task has older history than current state, cannot execute")
	}

	// Always add a WorkflowTaskStarted event before executing new tasks
	toExecute := []history.Event{e.createNewEvent(history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})}
	executedEvents := toExecute

	toExecute = append(toExecute, t.NewEvents...)

	// Execute new events received from the backend
	if !skipNewEvents {
		var err error
		executedEvents, err = e.executeNewEvents(toExecute)
		if err != nil {
			logger.Error("Error while executing new events", "error", err)

			e.workflowCompleted(nil, err)
		}
	}

	// Process any commands added while executing new events
	completed := false
	newCommandEvents := make([]history.Event, 0)
	activityEvents := make([]history.Event, 0)
	timerEvents := make([]history.Event, 0)
	workflowEvents := make([]history.WorkflowEvent, 0)

	for _, c := range e.workflowState.Commands() {
		if c.State() == command.CommandState_Done {
			continue
		}

		r := c.Execute(e.clock)
		if r == nil {
			continue
		}

		completed = completed || r.Completed
		newCommandEvents = append(newCommandEvents, r.Events...)
		activityEvents = append(activityEvents, r.ActivityEvents...)
		timerEvents = append(timerEvents, r.TimerEvents...)
		workflowEvents = append(workflowEvents, r.WorkflowEvents...)
	}

	// Events from commands don't have to be executed again, add them to the executed events.
	executedEvents = append(executedEvents, newCommandEvents...)

	// Set SequenceIDs for all executed events
	for i := range executedEvents {
		executedEvents[i].SequenceID = e.nextSequenceID()
	}

	logger.Debug("Finished workflow task",
		"executed", len(executedEvents),
		"last_sequence_id", e.lastSequenceID,
		"completed", completed,
	)

	return &ExecutionResult{
		Completed:      completed,
		Executed:       executedEvents,
		ActivityEvents: activityEvents,
		TimerEvents:    timerEvents,
		WorkflowEvents: workflowEvents,
	}, nil
}

func (e *executor) replayHistory(h []history.Event) error {
	e.workflowState.SetReplaying(true)
	for _, event := range h {
		if event.SequenceID < e.lastSequenceID {
			e.logger.Panic("history has older events than current state")
		}

		if err := e.executeEvent(event); err != nil {
			return err
		}

		// If we need to replay history before continuing execution of
		// a new task, the executor must know if WorkflowExecutionStarted
		// was seen during replay so it can determine if events should
		// be reordered before it starts executing events for the new task
		if event.Type == history.EventType_WorkflowExecutionStarted {
			e.wfStartedEventSeen = true
		}

		e.lastSequenceID = event.SequenceID
	}

	return nil
}

func (e *executor) executeNewEvents(newEvents []history.Event) ([]history.Event, error) {
	e.workflowState.SetReplaying(false)

	for i, event := range newEvents {
		if err := e.executeEvent(event); err != nil {
			return newEvents[:i], err
		}
	}

	if e.workflow.Completed() {
		// TODO: Is this too early? We haven't committed some of the commands
		if e.workflowState.HasPendingFutures() {
			e.logger.Panic("workflow completed, but there are still pending futures")
		}

		e.workflowCompleted(e.workflow.Result(), e.workflow.Error())
	}

	return newEvents, nil
}

func (e *executor) Close() {
	if e.workflow != nil {
		e.logger.Debug("Stopping workflow executor", "instance_id", e.workflowState.Instance().InstanceID)

		// End workflow if running to prevent leaking goroutines
		e.workflow.Close()
	}
}

func (e *executor) executeEvent(event history.Event) error {
	e.logger.Debug("Executing event",
		"instance_id", e.workflowState.Instance().InstanceID,
		"event_id", event.ID,
		"seq_id", event.SequenceID,
		"event_type", event.Type,
		"schedule_event_id", event.ScheduleEventID,
		"is_replaying", e.workflowState.Replaying(),
	)

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

	case history.EventType_TimerCanceled:
		err = e.handleTimerCanceled(event, event.Attributes.(*history.TimerCanceledAttributes))

	case history.EventType_SignalReceived:
		err = e.handleSignalReceived(event, event.Attributes.(*history.SignalReceivedAttributes))

	case history.EventType_SideEffectResult:
		err = e.handleSideEffectResult(event, event.Attributes.(*history.SideEffectResultAttributes))

	case history.EventType_SubWorkflowScheduled:
		err = e.handleSubWorkflowScheduled(event, event.Attributes.(*history.SubWorkflowScheduledAttributes))
	case history.EventType_SubWorkflowCancellationRequested:
		err = e.handleSubWorkflowCancellationRequest(event, event.Attributes.(*history.SubWorkflowCancellationRequestedAttributes))
	case history.EventType_SubWorkflowFailed:
		err = e.handleSubWorkflowFailed(event, event.Attributes.(*history.SubWorkflowFailedAttributes))
	case history.EventType_SubWorkflowCompleted:
		err = e.handleSubWorkflowCompleted(event, event.Attributes.(*history.SubWorkflowCompletedAttributes))

	case history.EventType_SignalWorkflow:
		err = e.handleSignalWorkflow(event, event.Attributes.(*history.SignalWorkflowAttributes))

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

	return e.workflow.Continue()
}

func (e *executor) handleWorkflowTaskStarted(event history.Event, a *history.WorkflowTaskStartedAttributes) error {
	e.workflowState.SetTime(event.Timestamp)

	return nil
}

func (e *executor) handleActivityScheduled(event history.Event, a *history.ActivityScheduledAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled an activity which could not be found")
	}

	sac, ok := c.(*command.ScheduleActivityCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution scheduled an activity, not: %v", c.Type())
	}

	// Ensure the same activity was scheduled again
	if a.Name != sac.Name {
		return fmt.Errorf("previous workflow execution scheduled different type of activity: %s, %s", a.Name, sac.Name)
	}

	c.Commit()

	return nil
}

func (e *executor) handleActivityCompleted(event history.Event, a *history.ActivityCompletedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return fmt.Errorf("could not find pending future for activity completion")
	}

	err := f(a.Result, nil)
	if err != nil {
		return fmt.Errorf("setting activity completed result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled an activity which could not be found")
	}

	sac, ok := c.(*command.ScheduleActivityCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution scheduled an activity, not: %v", c.Type())
	}

	sac.Done()

	return e.workflow.Continue()
}

func (e *executor) handleActivityFailed(event history.Event, a *history.ActivityFailedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future for activity failed event")
	}

	if err := f(nil, errors.New(a.Reason)); err != nil {
		return fmt.Errorf("setting activity failed result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled an activity which could not be found")
	}

	sac, ok := c.(*command.ScheduleActivityCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution scheduled an activity, not: %v", c.Type())
	}

	sac.Done()

	return e.workflow.Continue()
}

func (e *executor) handleTimerScheduled(event history.Event, a *history.TimerScheduledAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled a timer")
	}

	if _, ok := c.(*command.ScheduleTimerCommand); !ok {
		return fmt.Errorf("previous workflow execution scheduled a timer, not: %v", c.Type())
	}

	c.Commit()

	return nil
}

func (e *executor) handleTimerFired(event history.Event, a *history.TimerFiredAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		// Timer already canceled ignore
		return nil
	}

	if err := f(nil, nil); err != nil {
		return fmt.Errorf("setting timer fired result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("no command found for timer fired event")
	}

	if _, ok := c.(*command.ScheduleTimerCommand); !ok {
		return fmt.Errorf("schedule timer command not found, instead: %v", c.Type())
	}

	c.Done()

	return e.workflow.Continue()
}

func (e *executor) handleTimerCanceled(event history.Event, a *history.TimerCanceledAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution canceled a timer")
	}

	stc, ok := c.(*command.ScheduleTimerCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution canceled a timer, not: %v", c.Type())
	}

	stc.HandleCancel()

	// Cancel the pending future
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		// Timer already canceled ignore
		return nil
	}

	if err := f(nil, sync.Canceled); err != nil {
		return fmt.Errorf("setting timer canceled result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	return e.workflow.Continue()
}

func (e *executor) handleSubWorkflowScheduled(event history.Event, a *history.SubWorkflowScheduledAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled a sub workflow")
	}

	sswc, ok := c.(*command.ScheduleSubWorkflowCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution scheduled a sub workflow, not: %v", c.Type())
	}

	if a.Name != sswc.Name {
		return fmt.Errorf("previous workflow execution scheduled different type of sub workflow: %s, %s", a.Name, sswc.Name)
	}

	// If we are replaying this event, the command will have generated a new instance ID. Ensure we use the same one as
	// when the command was originally committed.
	sswc.Instance = a.SubWorkflowInstance

	c.Commit()

	return nil
}

func (e *executor) handleSubWorkflowCancellationRequest(event history.Event, a *history.SubWorkflowCancellationRequestedAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution")
	}

	sswc, ok := c.(*command.ScheduleSubWorkflowCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution, not: %v", c.Type())
	}

	sswc.HandleCancel()

	return e.workflow.Continue()
}

func (e *executor) handleSubWorkflowFailed(event history.Event, a *history.SubWorkflowFailedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future found for sub workflow failed event")
	}

	if err := f(nil, errors.New(a.Error)); err != nil {
		return fmt.Errorf("setting sub workflow failed result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		// TODO: Adjust
		return fmt.Errorf("previous workflow execution scheduled a sub-workflow execution")
	}

	if _, ok := c.(*command.ScheduleSubWorkflowCommand); !ok {
		// TODO: Adjust
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution, not: %v", c.Type())
	}

	c.Done()

	return e.workflow.Continue()
}

func (e *executor) handleSubWorkflowCompleted(event history.Event, a *history.SubWorkflowCompletedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future found for sub workflow completed event")
	}

	if err := f(a.Result, nil); err != nil {
		return fmt.Errorf("setting sub workflow completed result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		// TODO: Adjust
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution")
	}

	if _, ok := c.(*command.ScheduleSubWorkflowCommand); !ok {
		// TODO: Adjust
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution, not: %v", c.Type())
	}

	c.Done()

	return e.workflow.Continue()
}

func (e *executor) handleSignalReceived(event history.Event, a *history.SignalReceivedAttributes) error {
	// Send signal to workflow channel
	workflowstate.ReceiveSignal(e.workflowState, a.Name, a.Arg)

	return e.workflow.Continue()
}

func (e *executor) handleSignalWorkflow(event history.Event, a *history.SignalWorkflowAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution requested a signal")
	}

	sewc, ok := c.(*command.SignalWorkflowCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution requested to signal a workflow, not: %v", c.Type())
	}

	sewc.Done()

	return e.workflow.Continue()
}

func (e *executor) handleSideEffectResult(event history.Event, a *history.SideEffectResultAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled a side effect")
	}

	sec, ok := c.(*command.SideEffectCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution scheduled a side effect, not: %v", c.Type())
	}

	sec.Done()

	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future found for side effect result event")
	}

	if err := f(a.Result, nil); err != nil {
		return fmt.Errorf("setting side effect result result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	return e.workflow.Continue()
}

func (e *executor) workflowCompleted(result payload.Payload, err error) {
	eventId := e.workflowState.GetNextScheduleEventID()

	cmd := command.NewCompleteWorkflowCommand(eventId, e.workflowState.Instance(), result, err)
	e.workflowState.AddCommand(cmd)
}

func (e *executor) nextSequenceID() int64 {
	e.lastSequenceID++
	return e.lastSequenceID
}

func (e *executor) createNewEvent(eventType history.EventType, attributes interface{}, opts ...history.HistoryEventOption) history.Event {
	return history.NewPendingEvent(
		e.clock.Now(),
		eventType,
		attributes,
		opts...,
	)
}
