package testing

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/activity"
	margs "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/pkg/core"
	"github.com/cschleiden/go-workflows/pkg/core/task"
	"github.com/cschleiden/go-workflows/pkg/history"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type WorkflowTester interface {
	// Now returns the current time of the simulated clock in the tester
	Now() time.Time

	Execute(args ...interface{})

	Registry() *workflow.Registry

	OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call

	OnSubWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call

	// OnSignal() // TODO: Allow waiting

	SignalWorkflow(signalName string, value interface{})

	WorkflowFinished() bool

	WorkflowResult(vtpr interface{}, err *string)

	AssertExpectations(t *testing.T)

	ScheduleCallback(delay time.Duration, callback func())
}

type testTimer struct {
	// At is the timer this timer is scheduled for. This will advance the mock clock
	// to this timestamp
	At time.Time

	// Callback is called when the timer should fire. It can return a history event which
	// will be added to the event history being executed.
	Callback func()
}

type options struct {
	TestTimeout time.Duration
}

type workflowTester struct {
	options *options

	// Workflow under test
	wf  workflow.Workflow
	wfi core.WorkflowInstance

	workflowFinished bool
	workflowResult   payload.Payload
	workflowErr      string

	registry         *workflow.Registry
	ma               *mock.Mock
	mockedActivities map[string]bool
	mw               *mock.Mock

	clock *clock.Mock

	timers    []*testTimer
	callbacks chan func() *history.Event

	runningActivities int32
}

func NewWorkflowTester(wf workflow.Workflow) WorkflowTester {
	// Start with the current wall-clock tiem
	clock := clock.NewMock()
	clock.Set(time.Now())

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	registry := workflow.NewRegistry()

	wt := &workflowTester{
		options: &options{
			TestTimeout: time.Second * 10,
		},

		wf:       wf,
		wfi:      wfi,
		registry: registry,

		ma:               &mock.Mock{},
		mockedActivities: make(map[string]bool),
		mw:               &mock.Mock{},

		clock: clock,

		timers:    make([]*testTimer, 0),
		callbacks: make(chan func() *history.Event, 1024),
	}

	// Always register the workflow under test
	wt.registry.RegisterWorkflow(wf)

	return wt
}

func (wt *workflowTester) Now() time.Time {
	return wt.clock.Now()
}

func (wt *workflowTester) Registry() *workflow.Registry {
	return wt.registry
}

func (wt *workflowTester) ScheduleCallback(delay time.Duration, callback func()) {
	wt.timers = append(wt.timers, &testTimer{
		At:       wt.clock.Now().Add(delay),
		Callback: callback,
	})
}

func (wt *workflowTester) OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call {
	// Register activity so that we can correctly identify its arguments later
	wt.registry.RegisterActivity(activity)

	name := fn.Name(activity)
	call := wt.ma.On(name, args...)
	wt.mockedActivities[name] = true
	call.Return(nil)
	return call
}

func (wt *workflowTester) OnSubWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call {
	wt.registry.RegisterWorkflow(workflow)

	name := fn.Name(workflow)
	return wt.mw.On(name, args...)
}

func (wt *workflowTester) Execute(args ...interface{}) {
	task := wt.getInitialWorkflowTask(wt.wfi, wt.wf, args...)

	h := make([]history.Event, 0)

	for !wt.workflowFinished {
		e, err := workflow.NewExecutor(wt.registry, task.WorkflowInstance, wt.clock)
		if err != nil {
			panic("could not create workflow executor" + err.Error())
		}

		// TODO: Handle workflow events
		executedEvents, _ /*workflowEvents*/, err := e.ExecuteTask(context.Background(), task)
		if err != nil {
			panic("Error while executing workflow" + err.Error())
		}

		// Process events for next task
		newEvents := make([]history.Event, 0)

		for _, event := range executedEvents {
			switch event.Type {
			case history.EventType_WorkflowExecutionFinished:
				wt.workflowFinished = true
				wt.workflowResult = event.Attributes.(*history.ExecutionCompletedAttributes).Result
				wt.workflowErr = event.Attributes.(*history.ExecutionCompletedAttributes).Error

			case history.EventType_ActivityScheduled:
				wt.scheduleActivity(task.WorkflowInstance, event)

			case history.EventType_SubWorkflowScheduled:
				newEvents = append(newEvents, wt.getSubWorkflowResultEvent(event))

			case history.EventType_TimerScheduled:
				wt.scheduleTimer(event)
			}

			log.Println(event.Type.String())

			// TODO: SubWorkflows
		}

		for len(newEvents) == 0 && !wt.workflowFinished {
			// No new events left and the workflow isn't finished yet. Check for timers or callbacks

			select {
			case callback := <-wt.callbacks:
				event := callback()
				if event != nil {
					newEvents = append(newEvents, *event)
				}
				continue
			default:
			}

			if len(wt.timers) > 0 {
				// Take first timer and execute it
				sort.SliceStable(wt.timers, func(i, j int) bool {
					a := wt.timers[i]
					b := wt.timers[j]

					return a.At.Before(b.At)
				})

				t := wt.timers[0]
				wt.timers = wt.timers[1:]

				// Advance clock
				wt.clock.Set(t.At)

				t.Callback()
			} else {
				t := time.NewTimer(wt.options.TestTimeout)

				select {
				case callback := <-wt.callbacks:
					event := callback()
					if event != nil {
						newEvents = append(newEvents, *event)
					}
				case <-t.C:
					t.Stop()
					panic("No new events generated during workflow execution and no pending timers, workflow blocked?")
				}
			}
		}

		h = append(h, executedEvents...)
		task = getNextWorkflowTask(wt.wfi, h, newEvents)
	}
}

func (wt *workflowTester) SignalWorkflow(name string, value interface{}) {
	arg, err := converter.DefaultConverter.To(value)
	if err != nil {
		panic("Could not convert signal value to string" + err.Error())
	}

	wt.callbacks <- func() *history.Event {
		e := history.NewHistoryEvent(
			wt.clock.Now(),
			history.EventType_SignalReceived,
			-1,
			&history.SignalReceivedAttributes{
				Name: name,
				Arg:  arg,
			},
		)

		return &e
	}
}

func (wt *workflowTester) WorkflowFinished() bool {
	return wt.workflowFinished
}

func (wt *workflowTester) WorkflowResult(vtpr interface{}, err *string) {
	if wt.workflowErr == "" {
		if err := converter.DefaultConverter.From(wt.workflowResult, vtpr); err != nil {
			panic("Could not convert result to provided type" + err.Error())
		}
	}

	if err != nil {
		*err = wt.workflowErr
	}
}

func (wt *workflowTester) AssertExpectations(t *testing.T) {
	wt.ma.AssertExpectations(t)
}

func (wt *workflowTester) scheduleActivity(wfi core.WorkflowInstance, event history.Event) {
	e := event.Attributes.(*history.ActivityScheduledAttributes)

	// Execute real activity
	go func() {
		atomic.AddInt32(&wt.runningActivities, 1)
		defer atomic.AddInt32(&wt.runningActivities, -1)

		var activityErr error
		var activityResult payload.Payload

		// Execute mocked activity. If an activity is mocked once, we'll never fall back to the original implementation
		if wt.mockedActivities[e.Name] {
			afn, err := wt.registry.GetActivity(e.Name)
			if err != nil {
				panic("Could not find activity " + e.Name + " in registry")
			}

			argValues, addContext, err := margs.InputsToArgs(converter.DefaultConverter, reflect.ValueOf(afn), e.Inputs)
			if err != nil {
				panic("Could not convert activity inputs to args: " + err.Error())
			}

			args := make([]interface{}, len(argValues))
			for i, arg := range argValues {
				if i == 0 && addContext {
					args[i] = context.Background()
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
				activityResult, err = converter.DefaultConverter.To(result)
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
			executor := activity.NewExecutor(wt.registry)
			activityResult, activityErr = executor.ExecuteActivity(context.Background(), &task.Activity{
				ID:               uuid.NewString(),
				WorkflowInstance: wfi,
				Event:            event,
			})
		}

		wt.callbacks <- func() *history.Event {
			var ne history.Event

			if activityErr != nil {
				ne = history.NewHistoryEvent(
					wt.clock.Now(),
					history.EventType_ActivityFailed,
					event.EventID,
					&history.ActivityFailedAttributes{
						Reason: activityErr.Error(),
					},
				)
			} else {
				ne = history.NewHistoryEvent(
					wt.clock.Now(),
					history.EventType_ActivityCompleted,
					event.EventID,
					&history.ActivityCompletedAttributes{
						Result: activityResult,
					},
				)
			}

			return &ne
		}
	}()
}

func (wt *workflowTester) scheduleTimer(event history.Event) {
	e := event.Attributes.(*history.TimerScheduledAttributes)

	wt.timers = append(wt.timers, &testTimer{
		At: e.At,
		Callback: func() {
			wt.callbacks <- func() *history.Event {
				timerFiredEvent := history.NewFutureHistoryEvent(
					wt.clock.Now(),
					history.EventType_TimerFired,
					event.EventID,
					&history.TimerFiredAttributes{},
					e.At,
				)
				return &timerFiredEvent
			}
		},
	})
}

func (wt *workflowTester) getSubWorkflowResultEvent(event history.Event) history.Event {
	e := event.Attributes.(*history.SubWorkflowScheduledAttributes)

	// TODO: This only allows mocking and not executing the actual workflow
	results := wt.mw.MethodCalled(e.Name)

	var workflowErr error
	var workflowResult interface{}

	switch len(results) {
	case 1:
		// Expect only error
		workflowErr = results.Error(0)
		workflowResult = nil
	case 2:
		workflowResult = results.Get(0)
		workflowErr = results.Error(1)
	default:
		panic(
			fmt.Sprintf(
				"Unexpected number of results returned for workflow %v, expected 1 or 2, got %v",
				e.Name,
				len(results),
			),
		)
	}

	if workflowErr != nil {
		return history.NewHistoryEvent(
			wt.clock.Now(),
			history.EventType_SubWorkflowFailed,
			event.EventID,
			&history.SubWorkflowCompletedAttributes{
				Error: workflowErr.Error(),
			},
		)
	} else {
		result, err := converter.DefaultConverter.To(workflowResult)
		if err != nil {
			panic("Could not convert result for workflow " + e.Name + ": " + err.Error())
		}

		return history.NewHistoryEvent(
			wt.clock.Now(),
			history.EventType_SubWorkflowCompleted,
			event.EventID,
			&history.SubWorkflowCompletedAttributes{
				Result: result,
			},
		)
	}
}

func (wt *workflowTester) getInitialWorkflowTask(wfi core.WorkflowInstance, wf workflow.Workflow, args ...interface{}) *task.Workflow {
	name := fn.Name(wf)

	inputs, err := margs.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		panic(err)
	}

	event := history.NewHistoryEvent(
		wt.clock.Now(),
		history.EventType_WorkflowExecutionStarted,
		-1,
		&history.ExecutionStartedAttributes{
			Name:   name,
			Inputs: inputs,
		},
	)

	return &task.Workflow{
		WorkflowInstance: wfi,
		History:          []history.Event{},
		NewEvents:        []history.Event{event},
	}
}

func getNextWorkflowTask(wfi core.WorkflowInstance, history []history.Event, newEvents []history.Event) *task.Workflow {
	return &task.Workflow{
		WorkflowInstance: wfi,
		History:          history,
		NewEvents:        newEvents,
	}
}
