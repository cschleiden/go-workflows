package tester

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/activity"
	margs "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/signals"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace"
)

type testHistoryProvider struct {
	history []*history.Event
}

func (t *testHistoryProvider) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error) {
	return t.history, nil
}

type testTimer struct {
	// Instance is the workflow instance this timer is for
	Instance *core.WorkflowInstance

	// ScheduleEventID is the ID of the schedule event for this timer
	ScheduleEventID int64

	// At is the time this timer is scheduled for in test time
	At time.Time

	// Callback is called when the timer should fire.
	Callback *func()

	TimerEvent *history.WorkflowEvent

	wallClockTimer *clock.Timer
}

func (tt *testTimer) fire() *history.WorkflowEvent {
	if tt.Callback != nil {
		(*tt.Callback)()
		return nil
	}

	return tt.TimerEvent
}

type testWorkflow struct {
	instance      *core.WorkflowInstance
	metadata      *core.WorkflowMetadata
	history       []*history.Event
	pendingEvents []*history.Event
}

type WorkflowTester[TResult any] interface {
	// Now returns the current time of the simulated clock in the tester
	Now() time.Time

	Execute(ctx context.Context, args ...interface{})

	Registry() *workflow.Registry

	OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call

	OnActivityByName(name string, activity workflow.Activity, args ...interface{}) *mock.Call

	OnSubWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call

	OnSubWorkflowByName(name string, workflow workflow.Workflow, args ...interface{}) *mock.Call

	SignalWorkflow(signalName string, value interface{})

	SignalWorkflowInstance(wfi *core.WorkflowInstance, signalName string, value interface{}) error

	WorkflowFinished() bool

	WorkflowResult() (TResult, error)

	// AssertExpectations asserts any assertions set up for mock activities and sub-workflow
	AssertExpectations(t *testing.T)

	// ScheduleCallback schedules the given callback after the given delay in workflow time (not wall clock).
	ScheduleCallback(delay time.Duration, callback func())

	// ListenSubWorkflow registers a handler to be called when a sub-workflow is started.
	ListenSubWorkflow(listener func(instance *core.WorkflowInstance, name string))
}

type workflowTester[TResult any] struct {
	options *options

	// Workflow under test
	wf  interface{}
	wfi *core.WorkflowInstance
	wfm *core.WorkflowMetadata

	// Workflows
	mtw                       sync.RWMutex
	testWorkflowsByInstanceID map[string]*testWorkflow
	testWorkflows             []*testWorkflow

	workflowFinished bool
	workflowResult   payload.Payload
	workflowErr      *workflowerrors.Error

	registry *workflow.Registry

	ma               *mock.Mock
	mockedActivities map[string]bool

	mw              *mock.Mock
	mockedWorkflows map[string]bool

	workflowHistory []*history.Event
	clock           *clock.Mock
	wallClock       clock.Clock

	// Wall-clock start time of the workflow test run
	startTime time.Time

	timers         []*testTimer
	wallClockTimer *clock.Timer

	// timerWallClockStart time.Time
	timerMode timeMode

	callbacks chan func() *history.WorkflowEvent

	subWorkflowListener func(*core.WorkflowInstance, string)

	runningActivities int32

	logger *slog.Logger

	tracer trace.Tracer

	converter converter.Converter

	propagators []contextpropagation.ContextPropagator
}

var _ WorkflowTester[any] = (*workflowTester[any])(nil)

func NewWorkflowTester[TResult any](wf interface{}, opts ...WorkflowTesterOption) *workflowTester[TResult] {
	if err := margs.ReturnTypeMatch[TResult](wf); err != nil {
		panic(fmt.Sprintf("workflow return type does not match: %s", err))
	}

	// Start with the current wall-c time
	c := clock.NewMock()
	c.Set(time.Now())

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	registry := workflow.NewRegistry()

	options := &options{
		TestTimeout: time.Second * 10,
		Logger:      slog.Default(),
		Converter:   converter.DefaultConverter,
	}

	for _, o := range opts {
		o(options)
	}

	tracer := trace.NewNoopTracerProvider().Tracer("workflow-tester")

	wt := &workflowTester[TResult]{
		options: options,

		wf:       wf,
		wfi:      wfi,
		wfm:      &core.WorkflowMetadata{},
		registry: registry,

		testWorkflows:             make([]*testWorkflow, 0),
		testWorkflowsByInstanceID: make(map[string]*testWorkflow),

		ma:               &mock.Mock{},
		mockedActivities: make(map[string]bool),

		mw:              &mock.Mock{},
		mockedWorkflows: make(map[string]bool),

		workflowHistory: make([]*history.Event, 0),
		clock:           c,
		wallClock:       clock.New(),

		timers:    make([]*testTimer, 0),
		callbacks: make(chan func() *history.WorkflowEvent, 1024),
		timerMode: TM_TimeTravel,

		logger:      options.Logger.With("source", "tester"),
		tracer:      tracer,
		converter:   options.Converter,
		propagators: options.Propagators,
	}

	// Register internal activities
	signalActivities := &signals.Activities{Signaler: &signaler[TResult]{wt}}
	registry.RegisterActivity(signalActivities)

	// Always register the workflow under test
	if err := wt.registry.RegisterWorkflow(wf); err != nil {
		panic(fmt.Sprintf("could not register workflow under test: %v", err))
	}

	return wt
}

func (wt *workflowTester[TResult]) Now() time.Time {
	return wt.clock.Now()
}

func (wt *workflowTester[TResult]) Registry() *workflow.Registry {
	return wt.registry
}

func (wt *workflowTester[TResult]) ScheduleCallback(delay time.Duration, callback func()) {
	wt.timers = append(wt.timers, &testTimer{
		At:         wt.clock.Now().Add(delay),
		Callback:   &callback,
		TimerEvent: nil,
	})
}

func (wt *workflowTester[TResult]) ListenSubWorkflow(listener func(*core.WorkflowInstance, string)) {
	wt.subWorkflowListener = listener
}

func (wt *workflowTester[TResult]) OnActivityByName(name string, activity workflow.Activity, args ...interface{}) *mock.Call {
	// Register activity so that we can correctly identify its arguments later
	wt.registry.RegisterActivityByName(name, activity)

	wt.mockedActivities[name] = true
	return wt.ma.On(name, args...)
}

func (wt *workflowTester[TResult]) OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call {
	// Register activity so that we can correctly identify its arguments later
	wt.registry.RegisterActivity(activity)

	name := fn.Name(activity)
	wt.mockedActivities[name] = true
	return wt.ma.On(name, args...)
}

func (wt *workflowTester[TResult]) OnSubWorkflowByName(name string, workflow workflow.Workflow, args ...interface{}) *mock.Call {
	// Register workflow so that we can correctly identify its arguments later
	wt.registry.RegisterWorkflowByName(name, workflow)

	wt.mockedWorkflows[name] = true
	return wt.mw.On(name, args...)
}

func (wt *workflowTester[TResult]) OnSubWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call {
	// Register workflow so that we can correctly identify its arguments later
	wt.registry.RegisterWorkflow(workflow)

	name := fn.Name(workflow)
	wt.mockedWorkflows[name] = true
	return wt.mw.On(name, args...)
}

func (wt *workflowTester[TResult]) Execute(ctx context.Context, args ...interface{}) {
	for _, propagator := range wt.propagators {
		if err := propagator.Inject(ctx, wt.wfm); err != nil {
			panic(fmt.Errorf("failed to inject context: %w", err))
		}
	}

	// Record start time of test run
	wt.startTime = wt.clock.Now()

	// Start workflow under test
	initialEvent := wt.getInitialEvent(wt.wf, args)
	wt.addWorkflow(wt.wfi, wt.wfm, initialEvent)

	for !wt.workflowFinished {
		// Execute all workflows until no more events
		gotNewEvents := false

		for _, tw := range wt.testWorkflows {
			if len(tw.pendingEvents) == 0 {
				// Nothing to process for this workflow
				continue
			}

			// Get task
			t := getNextWorkflowTask(tw.instance, tw.history, tw.pendingEvents)
			tw.pendingEvents = tw.pendingEvents[:0]

			// Execute task
			e, err := workflow.NewExecutor(wt.logger, wt.tracer, wt.registry, wt.converter, wt.propagators, &testHistoryProvider{tw.history}, tw.instance, tw.metadata, wt.clock)
			if err != nil {
				panic(fmt.Errorf("could not create workflow executor: %v", err))
			}

			result, err := e.ExecuteTask(ctx, t)
			if err != nil {
				panic("Error while executing workflow" + err.Error())
			}

			e.Close()

			// Add all executed events to history
			tw.history = append(tw.history, result.Executed...)

			for _, event := range result.Executed {
				wt.logger.Debug("Event", log.EventTypeKey, event.Type)

				switch event.Type {
				case history.EventType_WorkflowExecutionFinished:
					a := event.Attributes.(*history.ExecutionCompletedAttributes)

					if !tw.instance.SubWorkflow() {
						wt.workflowFinished = true
						wt.workflowResult = a.Result
						wt.workflowErr = a.Error
					}

				case history.EventType_TimerCanceled:
					wt.cancelTimer(tw.instance, event)
				}
			}

			// Schedule sub-workflows and handle x-workflow events
			for _, workflowEvent := range result.WorkflowEvents {
				gotNewEvents = true
				wt.logger.Debug("Workflow event", log.EventTypeKey, workflowEvent.HistoryEvent.Type)

				switch workflowEvent.HistoryEvent.Type {
				case history.EventType_WorkflowExecutionStarted:
					wt.scheduleSubWorkflow(workflowEvent)

				default:
					wt.sendEvent(workflowEvent.WorkflowInstance, workflowEvent.HistoryEvent)
				}
			}

			// Schedule activities
			for _, event := range result.ActivityEvents {
				gotNewEvents = true

				a := event.Attributes.(*history.ActivityScheduledAttributes)
				wt.logger.Debug("Activity event", log.ActivityNameKey, a.Name)

				wt.scheduleActivity(tw.instance, tw.metadata, event)
			}

			// Schedule timers
			for _, timerEvent := range result.TimerEvents {
				gotNewEvents = true
				wt.logger.Debug("Timer future event", log.EventTypeKey, timerEvent.Type, log.AtKey, *timerEvent.VisibleAt)

				wt.scheduleTimer(tw.instance, timerEvent)
			}
		}

		for !wt.workflowFinished && !gotNewEvents {
			// No new events left and workflows aren't finished yet. Check for callbacks
			select {
			case callback := <-wt.callbacks:
				event := callback()
				if event != nil {
					wt.sendEvent(event.WorkflowInstance, event.HistoryEvent)
					gotNewEvents = true
				}
				continue
			default:
			}

			// No callbacks, try to fire any pending timers
			if wt.fireTimer() {
				// Timer fired
				continue
			}

			// Wait until a callback is ready or we hit the test/idle timeout
			t := time.NewTimer(wt.options.TestTimeout)

			select {
			case callback := <-wt.callbacks:
				event := callback()
				if event != nil {
					wt.sendEvent(event.WorkflowInstance, event.HistoryEvent)
					gotNewEvents = true
				}

			case <-t.C:
				t.Stop()
				panic("No new events generated during workflow execution and no pending timers, workflow blocked?")
			}
		}
	}
}

func (wt *workflowTester[TResult]) fireTimer() bool {
	if len(wt.timers) == 0 {
		// No timers to fire
		return false
	}

	// Determine mode we should be in and transition if it doesn't match the current one
	newMode := wt.newTimerMode()
	if wt.timerMode != newMode {
		wt.logger.Debug("Transitioning timer mode", log.TimerModeFrom, wt.timerMode, log.TimerModeTo, newMode)

		// Transition timer mode
		switch newMode {
		case TM_TimeTravel:
			if wt.wallClockTimer != nil {
				wt.wallClockTimer.Stop()
				wt.wallClockTimer = nil
			}

		case TM_WallClock:
			// Going from time-travel to wall-clock mode. Nothing to do here.
		}

		wt.timerMode = newMode
	}

	switch wt.timerMode {
	case TM_TimeTravel:
		{
			// Pop first timer and execute it
			t := wt.timers[0]
			wt.timers = wt.timers[1:]

			wt.logger.Debug("Advancing workflow clock to fire timer", log.ToKey, t.At)

			// Advance workflow clock and fire the timer
			wt.clock.Set(t.At)
			wt.callbacks <- t.fire
			return true
		}

	case TM_WallClock:
		{
			if wt.wallClockTimer != nil {
				// Wall-clock timer already scheduled
				return false
			}

			t := wt.timers[0]

			wt.logger.Debug("Scheduling wall-clock timer", log.AtKey, t.At)

			// Determine when this should run
			remainingTime := t.At.Sub(wt.clock.Now())

			// Schedule timer
			wt.wallClockTimer = wt.wallClock.AfterFunc(remainingTime, func() {
				wt.callbacks <- func() *history.WorkflowEvent {
					// Remove timer
					wt.timers = wt.timers[1:]
					wt.wallClockTimer = nil

					return t.fire()
				}
			})
		}
	}

	return false
}

func (wt *workflowTester[TResult]) newTimerMode() timeMode {
	runningActivities := atomic.LoadInt32(&wt.runningActivities)
	if runningActivities > 0 {
		return TM_WallClock
	}

	return TM_TimeTravel
}

func (wt *workflowTester[TResult]) sendEvent(wfi *core.WorkflowInstance, event *history.Event) {
	w := wt.getWorkflow(wfi)

	if w == nil {
		panic(fmt.Sprintf("tried to send event to instance %s which does not exist", wfi.InstanceID))
	}

	w.pendingEvents = append(w.pendingEvents, event)
}

func (wt *workflowTester[TResult]) SignalWorkflow(name string, value interface{}) {
	wt.SignalWorkflowInstance(wt.wfi, name, value)
}

func (wt *workflowTester[TResult]) SignalWorkflowInstance(wfi *core.WorkflowInstance, name string, value interface{}) error {
	if wt.getWorkflow(wfi) == nil {
		return backend.ErrInstanceNotFound
	}

	arg, err := wt.converter.To(value)
	if err != nil {
		panic("Could not convert signal value to string" + err.Error())
	}

	wt.callbacks <- func() *history.WorkflowEvent {
		e := history.NewPendingEvent(
			wt.clock.Now(),
			history.EventType_SignalReceived,
			&history.SignalReceivedAttributes{
				Name: name,
				Arg:  arg,
			},
		)

		return &history.WorkflowEvent{
			WorkflowInstance: wfi,
			HistoryEvent:     e,
		}
	}

	return nil
}

func (wt *workflowTester[TResult]) WorkflowFinished() bool {
	return wt.workflowFinished
}

func (wt *workflowTester[TResult]) WorkflowResult() (TResult, error) {
	var r TResult
	if wt.workflowResult != nil {
		if err := wt.converter.From(wt.workflowResult, &r); err != nil {
			panic("could not convert workflow result to expected type" + err.Error())
		}
	}

	err := workflowerrors.ToError(wt.workflowErr)
	return r, err
}

func (wt *workflowTester[TResult]) AssertExpectations(t *testing.T) {
	wt.ma.AssertExpectations(t)
}

func (wt *workflowTester[TResult]) scheduleActivity(wfi *core.WorkflowInstance, wfm *core.WorkflowMetadata, event *history.Event) {
	e := event.Attributes.(*history.ActivityScheduledAttributes)

	atomic.AddInt32(&wt.runningActivities, 1)

	go func() {
		defer atomic.AddInt32(&wt.runningActivities, -1)

		var activityErr error
		var activityResult payload.Payload

		// Execute mocked activity. If an activity is mocked once, we'll never fall back to the original implementation
		if wt.mockedActivities[e.Name] {
			afn, err := wt.registry.GetActivity(e.Name)
			if err != nil {
				panic("Could not find activity " + e.Name + " in registry")
			}

			argValues, addContext, err := margs.InputsToArgs(wt.converter, reflect.ValueOf(afn), e.Inputs)
			if err != nil {
				panic("Could not convert activity inputs to args: " + err.Error())
			}

			args := make([]interface{}, len(argValues))
			for i, arg := range argValues {
				if i == 0 && addContext {
					ctx := context.Background()

					for _, propagator := range wt.propagators {
						ctx, err = propagator.Extract(ctx, wfm)
						if err != nil {
							panic(fmt.Errorf("could not extract context from workflow metadata: %w", err))
						}
					}

					args[i] = ctx
					continue
				}

				args[i] = arg.Interface()
			}

			results := wt.ma.MethodCalled(e.Name, args...)

			switch len(results) {
			case 1:
				// Expect only error
				activityErr = results.Error(0)
				activityResult = nil
			case 2:
				result := results.Get(0)
				activityResult, err = wt.converter.To(result)
				if err != nil {
					panic("Could not convert result for activity " + e.Name + ": " + err.Error())
				}

				activityErr = results.Error(1)
			default:
				panic(
					fmt.Sprintf(
						"Unexpected number of results returned for mocked activity %v, expected 1 or 2, got %v",
						e.Name,
						len(results),
					),
				)
			}

		} else {
			executor := activity.NewExecutor(wt.logger, wt.tracer, wt.converter, wt.propagators, wt.registry)
			activityResult, activityErr = executor.ExecuteActivity(context.Background(), &task.Activity{
				ID:               uuid.NewString(),
				WorkflowInstance: wfi,
				Event:            event,
			})
		}

		wt.callbacks <- func() *history.WorkflowEvent {
			var ne *history.Event

			if activityErr != nil {
				aerr := workflowerrors.FromError(activityErr)

				ne = history.NewPendingEvent(
					wt.clock.Now(),
					history.EventType_ActivityFailed,
					&history.ActivityFailedAttributes{
						Error: aerr,
					},
					history.ScheduleEventID(event.ScheduleEventID),
				)
			} else {
				ne = history.NewPendingEvent(
					wt.clock.Now(),
					history.EventType_ActivityCompleted,
					&history.ActivityCompletedAttributes{
						Result: activityResult,
					},
					history.ScheduleEventID(event.ScheduleEventID),
				)
			}

			return &history.WorkflowEvent{
				WorkflowInstance: wfi,
				HistoryEvent:     ne,
			}
		}
	}()
}

func (wt *workflowTester[TResult]) scheduleTimer(instance *core.WorkflowInstance, event *history.Event) {
	e := event.Attributes.(*history.TimerFiredAttributes)

	wt.timers = append(wt.timers, &testTimer{
		Instance:        instance,
		ScheduleEventID: event.ScheduleEventID,
		At:              e.At,
		TimerEvent: &history.WorkflowEvent{
			WorkflowInstance: instance,
			HistoryEvent:     event,
		},
	})

	sort.SliceStable(wt.timers, func(i, j int) bool {
		return wt.timers[i].At.Before(wt.timers[j].At)
	})
}

func (wt *workflowTester[TResult]) cancelTimer(instance *core.WorkflowInstance, event *history.Event) {
	for i, t := range wt.timers {
		if t.Instance != nil && t.Instance.InstanceID == instance.InstanceID && t.ScheduleEventID == event.ScheduleEventID {
			// If this was the next timer to fire, stop the timer
			if t.wallClockTimer != nil {
				t.wallClockTimer.Stop()
			}

			wt.timers = append(wt.timers[:i], wt.timers[i+1:]...)
			break
		}
	}
}

func (wt *workflowTester[TResult]) getWorkflow(instance *core.WorkflowInstance) *testWorkflow {
	wt.mtw.RLock()
	defer wt.mtw.RUnlock()

	return wt.testWorkflowsByInstanceID[instance.InstanceID]
}

func (wt *workflowTester[TResult]) addWorkflow(instance *core.WorkflowInstance, metadata *core.WorkflowMetadata, initialEvent *history.Event) *testWorkflow {
	wt.mtw.Lock()
	defer wt.mtw.Unlock()

	tw := &testWorkflow{
		instance:      instance,
		metadata:      metadata,
		pendingEvents: []*history.Event{initialEvent},
		history:       make([]*history.Event, 0),
	}
	wt.testWorkflows = append(wt.testWorkflows, tw)
	wt.testWorkflowsByInstanceID[instance.InstanceID] = tw

	return tw
}

func (wt *workflowTester[TResult]) scheduleSubWorkflow(event history.WorkflowEvent) {
	a := event.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)

	// TODO: Right location to call handler?
	if wt.subWorkflowListener != nil {
		wt.subWorkflowListener(event.WorkflowInstance, a.Name)
	}

	wfn, err := wt.registry.GetWorkflow(a.Name)
	if err != nil {
		panic("Could not find workflow " + a.Name + " in registry")
	}

	argValues, addContext, err := margs.InputsToArgs(wt.converter, reflect.ValueOf(wfn), a.Inputs)
	if err != nil {
		panic("Could not convert workflow inputs to args: " + err.Error())
	}

	args := make([]interface{}, len(argValues))
	for i, arg := range argValues {
		if i == 0 && addContext {
			args[i] = context.Background()
			continue
		}

		args[i] = arg.Interface()
	}

	if !wt.mockedWorkflows[a.Name] {
		// Workflow not mocked, allow event to be processed
		wt.addWorkflow(event.WorkflowInstance, a.Metadata, event.HistoryEvent)
		return
	}

	var workflowRawErr error
	var workflowResult payload.Payload

	results := wt.mw.MethodCalled(a.Name, args...)

	switch len(results) {
	case 1:
		// Expect only error
		workflowRawErr = results.Error(0)
		workflowResult = nil
	case 2:
		result := results.Get(0)
		workflowResult, err = wt.converter.To(result)
		if err != nil {
			panic("Could not convert result for mocked workflow " + a.Name + ": " + err.Error())
		}

		workflowRawErr = results.Error(1)
	default:
		panic(
			fmt.Sprintf(
				"Unexpected number of results returned for mocked workflow %v, expected 1 or 2, got %v",
				a.Name,
				len(results),
			),
		)
	}

	wt.callbacks <- func() *history.WorkflowEvent {
		r := command.NewCompleteWorkflowCommand(
			0, event.WorkflowInstance, workflowResult, workflowerrors.FromError(workflowRawErr),
		).Execute(wt.clock)

		return &r.WorkflowEvents[0]
	}
}

func (wt *workflowTester[TResult]) getInitialEvent(wf interface{}, args []interface{}) *history.Event {
	name := fn.Name(wf)

	inputs, err := margs.ArgsToInputs(wt.converter, args...)
	if err != nil {
		panic(err)
	}

	return history.NewHistoryEvent(
		1,
		wt.clock.Now(),
		history.EventType_WorkflowExecutionStarted,
		&history.ExecutionStartedAttributes{
			Name:     name,
			Metadata: &core.WorkflowMetadata{},
			Inputs:   inputs,
		},
	)
}

func getNextWorkflowTask(wfi *core.WorkflowInstance, history []*history.Event, newEvents []*history.Event) *task.Workflow {
	var lastSequenceID int64
	if len(history) > 0 {
		lastSequenceID = history[len(history)-1].SequenceID
	}

	return &task.Workflow{
		WorkflowInstance: wfi,
		Metadata:         &core.WorkflowMetadata{},
		LastSequenceID:   lastSequenceID,
		NewEvents:        newEvents,
	}
}

type signaler[T any] struct {
	wt *workflowTester[T]
}

func (s *signaler[T]) SignalWorkflow(ctx context.Context, instanceID string, name string, arg interface{}) error {
	return s.wt.SignalWorkflowInstance(core.NewWorkflowInstance(instanceID, ""), name, arg)
}

var _ signals.Signaler = (*signaler[any])(nil)
