package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/continueasnew"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/registry"
	wf "github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ExecutionResult struct {
	// New state of the workflow instance
	State core.WorkflowInstanceState

	// Events executed during the tastk execution
	Executed []*history.Event

	// Activities that were scheduled
	ActivityEvents []*history.Event

	// Timers that were scheduled
	TimerEvents []*history.Event

	// Events for other workflow instances
	WorkflowEvents []*history.WorkflowEvent
}

type WorkflowHistoryProvider interface {
	GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error)
}

type WorkflowExecutor interface {
	ExecuteTask(ctx context.Context, t *backend.WorkflowTask) (*ExecutionResult, error)

	Close()
}

type executor struct {
	registry          *registry.Registry
	historyProvider   WorkflowHistoryProvider
	workflow          *workflow
	workflowName      string
	workflowState     *workflowstate.WfState
	workflowCtx       sync.Context
	workflowCtxCancel sync.CancelFunc
	cv                converter.Converter
	clock             clock.Clock
	logger            *slog.Logger
	tracer            trace.Tracer
	lastSequenceID    int64

	workflowSpan trace.Span

	maxHistorySize int64
}

func NewExecutor(
	logger *slog.Logger,
	tracer trace.Tracer,
	r *registry.Registry,
	cv converter.Converter,
	propagators []wf.ContextPropagator,
	historyProvider WorkflowHistoryProvider,
	instance *core.WorkflowInstance,
	metadata *metadata.WorkflowMetadata,
	clock clock.Clock,
	maxHistorySize int64,
) (WorkflowExecutor, error) {
	s := workflowstate.NewWorkflowState(instance, logger, tracer, clock)

	wfCtx := sync.Background()
	wfCtx = contextvalue.WithConverter(wfCtx, cv)
	wfCtx = workflowstate.WithWorkflowState(wfCtx, s)
	wfCtx = sync.WithValue(wfCtx, contextvalue.PropagatorsCtxKey, propagators)
	wfCtx = contextvalue.WithRegistry(wfCtx, r)

	wfCtx, cancel := sync.WithCancel(wfCtx)

	// As part of this, the default tracing propagator will run, and set the parent span
	// in the context, which will be picked up by our workflow span later.
	for _, propagator := range propagators {
		var err error
		wfCtx, err = propagator.ExtractToWorkflow(wfCtx, metadata)
		if err != nil {
			return nil, fmt.Errorf("extracting context: %w", err)
		}
	}

	return &executor{
		registry:          r,
		historyProvider:   historyProvider,
		workflowState:     s,
		workflowCtx:       wfCtx,
		workflowCtxCancel: cancel,
		cv:                cv,
		clock:             clock,
		maxHistorySize:    maxHistorySize,
		logger:            logger,
		tracer:            tracer,
	}, nil
}

func (e *executor) ExecuteTask(ctx context.Context, t *backend.WorkflowTask) (*ExecutionResult, error) {
	logger := e.logger.With(
		log.TaskIDKey, t.ID,
	)

	logger.Debug("Executing workflow task", slog.Int64(log.TaskLastSequenceIDKey, t.LastSequenceID))

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

	skipNewEvents, err := e.catchupOnHistory(ctx, t, logger)
	if err != nil {
		return nil, err
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

			// Transition workflow to error state
			e.workflowCompleted(nil, err)
		}
	}

	// Enforce max history size limit
	if e.lastSequenceID+int64(len(executedEvents)) >= e.maxHistorySize {
		e.workflowCompleted(nil, fmt.Errorf("workflow history size exceeded %d events", e.maxHistorySize))
	}

	// Process any commands added while executing new events
	state := core.WorkflowInstanceStateActive
	newCommandEvents := make([]*history.Event, 0)
	activityEvents := make([]*history.Event, 0)
	timerEvents := make([]*history.Event, 0)
	workflowEvents := make([]*history.WorkflowEvent, 0)

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

	e.workflowState.SetHistoryLength(e.lastSequenceID)

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

func (e *executor) catchupOnHistory(ctx context.Context, t *backend.WorkflowTask, logger *slog.Logger) (bool, error) {
	if t.LastSequenceID < e.lastSequenceID {
		return false, fmt.Errorf("task has older history than current state, cannot execute")
	}

	if t.LastSequenceID > e.lastSequenceID {
		logger.Debug("Task has newer history than current state, fetching and replaying history",
			log.TaskSequenceIDKey, t.LastSequenceID,
			log.LocalSequenceIDKey, e.lastSequenceID)

		h, err := e.historyProvider.GetWorkflowInstanceHistory(ctx, t.WorkflowInstance, &e.lastSequenceID)
		if err != nil {
			return false, fmt.Errorf("getting workflow history: %w", err)
		}

		if err := e.replayHistory(h); err != nil {
			logger.Error("Error while replaying history", "error", err)

			// Fail workflow with an error. Skip executing new events, but still go through the commands
			e.workflowCompleted(nil, err)

			// With an error occurred during replay, we need to ensure new events don't get duplicate sequence ids
			e.lastSequenceID = t.LastSequenceID

			return true, nil
		} else if t.LastSequenceID != e.lastSequenceID {
			logger.Error("After replaying history, task still has newer history than current state",
				log.TaskSequenceIDKey, t.LastSequenceID,
				log.LocalSequenceIDKey, e.lastSequenceID)

			return false, errors.New("even after fetching history and replaying history executor state does not match task")
		}
	}

	return false, nil
}

func (e *executor) replayHistory(h []*history.Event) error {
	e.workflowState.SetReplaying(true)
	for _, event := range h {
		if event.SequenceID < e.lastSequenceID {
			e.logger.Error("history has older events than current state")
			return errors.New("history has older events than current state")
		}

		// Note: lastSequenceID is updated below after successful event execution.
		// For consistent history length reporting (e.g., for workflow code), we intentionally set historyLength here before executing the event.
		e.workflowState.SetHistoryLength(e.lastSequenceID + 1)

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
		// Update history length BEFORE executing the event to reflect the event about to be added
		e.workflowState.SetHistoryLength(e.lastSequenceID + int64(i) + 1)

		if err := e.executeEvent(event); err != nil {
			return newEvents[:i], err
		}
	}

	if e.workflow.Completed() {
		defer e.workflowSpan.End()

		if e.workflowState.HasPendingFutures() {
			// This should not happen, provide debug information to the developer
			var pending []string
			pf := e.workflowState.PendingFutureNames()
			for id, name := range pf {
				pending = append(pending, fmt.Sprintf("%d-%s", id, name))
			}
			slices.Sort(pending)

			if testing.Testing() {
				panic(fmt.Sprintf("workflow completed, but there are still pending futures: %s", pending))
			}

			return newEvents, tracing.WithSpanError(
				e.workflowSpan, fmt.Errorf("workflow completed, but there are still pending futures: %s", pending))
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
		e.logger.Debug("Stopping workflow executor")

		// End workflow if running to prevent leaking goroutines
		e.workflow.Close()
	}
}

func (e *executor) executeEvent(event *history.Event) error {
	fields := []any{
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
		err = e.handleWorkflowExecutionStarted(event, event.Attributes.(*history.ExecutionStartedAttributes))

	case history.EventType_WorkflowExecutionFinished:
	// Ignore

	case history.EventType_WorkflowExecutionContinuedAsNew:
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

	case history.EventType_TraceStarted:
		err = e.handleTraceStarted(event, event.Attributes.(*history.TraceStartedAttributes))

	default:
		return fmt.Errorf("unknown event type: %v", event.Type)
	}

	return err
}

func (e *executor) handleWorkflowExecutionStarted(event *history.Event, a *history.ExecutionStartedAttributes) error {
	e.workflowName = a.Name

	wfFn, err := e.registry.GetWorkflow(a.Name)
	if err != nil {
		return fmt.Errorf("workflow %s not found", a.Name)
	}

	// Set the parent span here, so we can associate the workflow span with its parent
	parentSpan := tracing.SpanFromContext(e.workflowCtx)
	ctx := trace.ContextWithSpan(context.Background(), parentSpan)

	span := tracing.SpanWithStartTime(
		ctx,
		e.tracer,
		tracing.WorkflowSpanName(e.workflowName),
		a.WorkflowSpanID,
		event.Timestamp)

	// Set in context for workflow execution
	e.workflowCtx = tracing.ContextWithSpan(e.workflowCtx, span)
	e.workflowSpan = span

	e.workflow = newWorkflow(reflect.ValueOf(wfFn))
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

	sac.Commit()

	return nil
}

func (e *executor) handleActivityCompleted(event *history.Event, a *history.ActivityCompletedAttributes) error {
	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return fmt.Errorf("could not find pending future for activity completion")
	}

	err := f.Set(a.Result, nil)
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
	if err := f.Set(nil, actErr); err != nil {
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

	if !e.workflowState.Replaying() {
		// Trace timer fired
		spanName := "Timer"
		if a.Name != "" {
			spanName = spanName + ": " + a.Name
		}

		sctx := context.Background()
		if a.TraceContext != nil {
			sctx = tracing.SpanContextFromContext(sctx, a.TraceContext)
		}
		_, span := e.tracer.Start(sctx, spanName, trace.WithAttributes(
			attribute.Int64(log.DurationKey, int64(a.At.Sub(a.ScheduledAt)/time.Millisecond)),
			attribute.String(log.NowKey, a.ScheduledAt.String()),
			attribute.String(log.AtKey, a.At.String()),
		), trace.WithTimestamp(a.ScheduledAt))
		span.End()
	}

	if err := f.Set(nil, nil); err != nil {
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

	if err := f.Set(nil, sync.Canceled); err != nil {
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

	if err := f.Set(nil, wfErr); err != nil {
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

	if err := f.Set(a.Result, nil); err != nil {
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

	if err := f.Set(a.Result, nil); err != nil {
		return fmt.Errorf("setting side effect result: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	return e.workflow.Continue()
}

func (e *executor) handleTraceStarted(event *history.Event, a *history.TraceStartedAttributes) error {
	c := e.workflowState.CommandByScheduleEventID(event.ScheduleEventID)
	if c == nil {
		return fmt.Errorf("previous workflow execution started a trace")
	}

	stc, ok := c.(*command.StartTraceCommand)
	if !ok {
		return fmt.Errorf("previous workflow execution started a trace, not: %v", c.Type())
	}

	stc.Done()

	f, ok := e.workflowState.FutureByScheduleEventID(event.ScheduleEventID)
	if !ok {
		return errors.New("no pending future found for start trace event")
	}

	if err := f.Set(a.SpanID, nil); err != nil {
		return fmt.Errorf("setting start trace spanID: %w", err)
	}

	e.workflowState.RemoveFuture(event.ScheduleEventID)

	return e.workflow.Continue()
}

func (e *executor) workflowCompleted(result payload.Payload, wfErr error) {
	eventId := e.workflowState.GetNextScheduleEventID()

	cmd := command.NewCompleteWorkflowCommand(eventId, e.workflowState.Instance(), result, workflowerrors.FromError(wfErr))
	e.workflowState.AddCommand(cmd)
}

func (e *executor) workflowRestarted(result payload.Payload, continueAsNew *continueasnew.Error) {
	eventId := e.workflowState.GetNextScheduleEventID()

	cmd := command.NewContinueAsNewCommand(
		eventId, e.workflowState.Instance(), result, e.workflowName, continueAsNew.Metadata, continueAsNew.Inputs)
	e.workflowState.AddCommand(cmd)

	e.workflowSpan.SetAttributes(
		attribute.String(log.ContinuedExecutionIDKey, cmd.ContinuedExecutionID),
	)
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
