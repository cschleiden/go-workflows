package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/continueasnew"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"github.com/cschleiden/go-workflows/log"
	"go.opentelemetry.io/otel/trace"
)

type ExecutionResult struct {
	State          core.WorkflowInstanceState
	Executed       []*history.Event
	ActivityEvents []*history.Event
	TimerEvents    []*history.Event
	WorkflowEvents []history.WorkflowEvent
}

type WorkflowHistoryProvider interface {
	GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error)
}

type WorkflowExecutor interface {
	ExecuteTask(ctx context.Context, t *task.Workflow) (*ExecutionResult, error)

	Close()
}

type executor struct {
	registry          *Registry
	historyProvider   WorkflowHistoryProvider
	workflow          *workflow
	workflowName      string
	workflowTracer    *workflowtracer.WorkflowTracer
	workflowState     *workflowstate.WfState
	workflowCtx       sync.Context
	workflowCtxCancel sync.CancelFunc
	cv                converter.Converter
	clock             clock.Clock
	logger            *slog.Logger
	tracer            trace.Tracer
	lastSequenceID    int64
	parentSpan        trace.Span
}

func NewExecutor(
	logger *slog.Logger,
	tracer trace.Tracer,
	registry *Registry,
	cv converter.Converter,
	propagators []contextpropagation.ContextPropagator,
	historyProvider WorkflowHistoryProvider,
	instance *core.WorkflowInstance,
	metadata *core.WorkflowMetadata,
	clock clock.Clock,
) (WorkflowExecutor, error) {
	s := workflowstate.NewWorkflowState(instance, logger, clock)

	wfTracer := workflowtracer.New(tracer)

	wfCtx := sync.Background()
	wfCtx = converter.WithConverter(wfCtx, cv)
	wfCtx = workflowtracer.WithWorkflowTracer(wfCtx, wfTracer)
	wfCtx = workflowstate.WithWorkflowState(wfCtx, s)
	wfCtx = contextpropagation.WithPropagators(wfCtx, propagators)
	wfCtx, cancel := sync.WithCancel(wfCtx)

	for _, propagator := range propagators {
		var err error
		wfCtx, err = propagator.ExtractToWorkflow(wfCtx, metadata)
		if err != nil {
			return nil, fmt.Errorf("extracting context: %w", err)
		}
	}

	// Get span from the workflow context, set by the default context propagator
	parentSpan := workflowtracer.SpanFromContext(wfCtx)

	return &executor{
		registry:          registry,
		historyProvider:   historyProvider,
		workflowTracer:    wfTracer,
		workflowState:     s,
		workflowCtx:       wfCtx,
		workflowCtxCancel: cancel,
		cv:                cv,
		clock:             clock,
		logger:            logger,
		tracer:            tracer,
		parentSpan:        parentSpan,
	}, nil
}

func (e *executor) ExecuteTask(ctx context.Context, t *task.Workflow) (*ExecutionResult, error) {
	logger := e.logger.With(
		log.TaskIDKey, t.ID,
		log.InstanceIDKey, t.WorkflowInstance.InstanceID,
		log.ExecutionIDKey, t.WorkflowInstance.ExecutionID,
	)

	logger.Debug("Executing workflow task", log.TaskLastSequenceIDKey, t.LastSequenceID)

	if t.WorkflowInstanceState == core.WorkflowInstanceStateFinished {
		// This could happen if signals are delivered after the workflow is finished
		logger.Error("Received workflow task for finished workflow instance, discarding events")

		// Log events that caused this task to be scheduled
		for _, event := range t.NewEvents {
			logger.Debug("Discarded event:",
				log.EventIDKey, event.ID,
				log.EventTypeKey, event.Type.String(),
				log.ScheduleEventIDKey, event.ScheduleEventID)
		}

		return &ExecutionResult{
			State: core.WorkflowInstanceStateFinished,
		}, nil
	}

	skipNewEvents := false

	if t.LastSequenceID > e.lastSequenceID {
		logger.Debug("Task has newer history than current state, fetching and replaying history",
			log.TaskSequenceIDKey, t.LastSequenceID,
			log.LocalSequenceIDKey, e.lastSequenceID)

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
			logger.Error("After replaying history, task still has newer history than current state",
				log.TaskSequenceIDKey, t.LastSequenceID,
				log.LocalSequenceIDKey, e.lastSequenceID)

			return nil, errors.New("even after fetching history and replaying history executor state does not match task")
		}
	} else if t.LastSequenceID < e.lastSequenceID {
		return nil, fmt.Errorf("task has older history than current state, cannot execute")
	}

	// Always add a WorkflowTaskStarted event before executing new tasks
	toExecute := []*history.Event{e.createNewEvent(history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})}
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
	state := core.WorkflowInstanceStateActive
	newCommandEvents := make([]*history.Event, 0)
	activityEvents := make([]*history.Event, 0)
	timerEvents := make([]*history.Event, 0)
	workflowEvents := make([]history.WorkflowEvent, 0)

	for _, c := range e.workflowState.Commands() {
		if c.State() == command.CommandState_Done {
			continue
		}

		r := c.Execute(e.clock)
		if r == nil {
			continue
		}

		if r.State > state {
			state = r.State
		}
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
		log.ExecutedEventsKey, len(executedEvents),
		log.TaskLastSequenceIDKey, e.lastSequenceID,
		log.WorkflowInstanceStateKey, state,
	)

	return &ExecutionResult{
		State:          state,
		Executed:       executedEvents,
		ActivityEvents: activityEvents,
		TimerEvents:    timerEvents,
		WorkflowEvents: workflowEvents,
	}, nil
}

func (e *executor) replayHistory(h []*history.Event) error {
	e.workflowState.SetReplaying(true)
	for _, event := range h {
		if event.SequenceID < e.lastSequenceID {
			e.logger.Error("history has older events than current state")
			panic("history has older events than current state")
		}

		if err := e.executeEvent(event); err != nil {
			return err
		}

		e.lastSequenceID = event.SequenceID
	}

	return nil
}

func (e *executor) executeNewEvents(newEvents []*history.Event) ([]*history.Event, error) {
	e.workflowState.SetReplaying(false)

	for i, event := range newEvents {
		if err := e.executeEvent(event); err != nil {
			return newEvents[:i], err
		}
	}

	if e.workflow.Completed() {
		// TODO: Is this too early? We haven't committed some of the commands
		if e.workflowState.HasPendingFutures() {
			e.logger.Error("workflow completed, but there are still pending futures")
			panic("workflow completed, but there are still pending futures")
		}

		if canErr, ok := e.workflow.Error().(*continueasnew.Error); ok {
			e.workflowRestarted(e.workflow.Result(), canErr)
		} else {
			e.workflowCompleted(e.workflow.Result(), e.workflow.Error())
		}
	}

	return newEvents, nil
}

func (e *executor) Close() {
	if e.workflow != nil {
		e.logger.Debug("Stopping workflow executor", log.InstanceIDKey, e.workflowState.Instance().InstanceID)

		// End workflow if running to prevent leaking goroutines
		e.workflow.Close()
	}
}

func (e *executor) executeEvent(event *history.Event) error {
	fields := []any{
		log.InstanceIDKey, e.workflowState.Instance().InstanceID,
		log.EventIDKey, event.ID,
		log.SeqIDKey, event.SequenceID,
		log.EventTypeKey, event.Type,
		log.ScheduleEventIDKey, event.ScheduleEventID,
		log.IsReplayingKey, e.workflowState.Replaying(),
	}

	attributesFields := getAttributesLoggingFields(event)
	if attributesFields != nil {
		fields = append(fields, attributesFields...)
	}

	e.logger.Debug("Executing event", fields...)

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

	default:
		return fmt.Errorf("unknown event type: %v", event.Type)
	}

	return err
}

func (e *executor) handleWorkflowExecutionStarted(a *history.ExecutionStartedAttributes) error {
	e.workflowName = a.Name

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

func (e *executor) handleWorkflowTaskStarted(event *history.Event, a *history.WorkflowTaskStartedAttributes) error {
	e.workflowState.SetTime(event.Timestamp)

	return nil
}

func (e *executor) handleActivityScheduled(event *history.Event, a *history.ActivityScheduledAttributes) error {
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

func (e *executor) handleActivityCompleted(event *history.Event, a *history.ActivityCompletedAttributes) error {
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

func (e *executor) handleActivityFailed(event *history.Event, a *history.ActivityFailedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future for activity failed event")
	}

	actErr := workflowerrors.ToError(a.Error)
	if err := f(nil, actErr); err != nil {
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

func (e *executor) handleTimerScheduled(event *history.Event, a *history.TimerScheduledAttributes) error {
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

func (e *executor) handleTimerFired(event *history.Event, a *history.TimerFiredAttributes) error {
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

func (e *executor) handleTimerCanceled(event *history.Event, a *history.TimerCanceledAttributes) error {
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

func (e *executor) handleSubWorkflowScheduled(event *history.Event, a *history.SubWorkflowScheduledAttributes) error {
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

func (e *executor) handleSubWorkflowCancellationRequest(event *history.Event, a *history.SubWorkflowCancellationRequestedAttributes) error {
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

func (e *executor) handleSubWorkflowFailed(event *history.Event, a *history.SubWorkflowFailedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future found for sub workflow failed event")
	}

	wfErr := workflowerrors.ToError(a.Error)

	if err := f(nil, wfErr); err != nil {
		return fmt.Errorf("setting sub workflow failed result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution scheduled a sub-workflow execution")
	}

	if _, ok := c.(*command.ScheduleSubWorkflowCommand); !ok {
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution, not: %v", c.Type())
	}

	c.Done()

	return e.workflow.Continue()
}

func (e *executor) handleSubWorkflowCompleted(event *history.Event, a *history.SubWorkflowCompletedAttributes) error {
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
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution")
	}

	if _, ok := c.(*command.ScheduleSubWorkflowCommand); !ok {
		return fmt.Errorf("previous workflow execution cancelled a sub-workflow execution, not: %v", c.Type())
	}

	c.Done()

	return e.workflow.Continue()
}

func (e *executor) handleSignalReceived(event *history.Event, a *history.SignalReceivedAttributes) error {
	// Send signal to workflow channel
	workflowstate.ReceiveSignal(e.workflowState, a.Name, a.Arg)

	return e.workflow.Continue()
}

func (e *executor) handleSideEffectResult(event *history.Event, a *history.SideEffectResultAttributes) error {
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

func (e *executor) workflowCompleted(result payload.Payload, wfErr error) error {
	eventId := e.workflowState.GetNextScheduleEventID()

	cmd := command.NewCompleteWorkflowCommand(eventId, e.workflowState.Instance(), result, workflowerrors.FromError(wfErr))
	e.workflowState.AddCommand(cmd)

	return nil
}

func (e *executor) workflowRestarted(result payload.Payload, continueAsNew *continueasnew.Error) {
	eventId := e.workflowState.GetNextScheduleEventID()

	cmd := command.NewContinueAsNewCommand(eventId, e.workflowState.Instance(), result, e.workflowName, continueAsNew.Metadata, continueAsNew.Inputs)
	e.workflowState.AddCommand(cmd)
}

func (e *executor) nextSequenceID() int64 {
	e.lastSequenceID++
	return e.lastSequenceID
}

func (e *executor) createNewEvent(eventType history.EventType, attributes interface{}, opts ...history.HistoryEventOption) *history.Event {
	return history.NewPendingEvent(
		e.clock.Now(),
		eventType,
		attributes,
		opts...,
	)
}

func getAttributesLoggingFields(event *history.Event) []any {
	switch event.Type {
	case history.EventType_WorkflowExecutionStarted:
		attributes := event.Attributes.(*history.ExecutionStartedAttributes)
		return []any{
			log.WorkflowNameKey, attributes.Name,
		}
	case history.EventType_SubWorkflowScheduled:
		attributes := event.Attributes.(*history.SubWorkflowScheduledAttributes)
		return []any{
			log.WorkflowNameKey, attributes.Name,
		}
	case history.EventType_SignalReceived:
		attributes := event.Attributes.(*history.SignalReceivedAttributes)
		return []any{
			log.SignalNameKey, attributes.Name,
		}
	case history.EventType_ActivityScheduled:
		attributes := event.Attributes.(*history.ActivityScheduledAttributes)
		return []any{
			log.ActivityNameKey, attributes.Name,
		}
	default:
		return nil
	}
}
