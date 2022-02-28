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

	// TODO: Allow sending signals to other workflows
	SignalWorkflow(signalName string, value interface{})

	WorkflowFinished() bool

	WorkflowResult(vtpr interface{}, err *string)

	// AssertExpectations asserts any assertions set up for mock activities and sub-workflow
	AssertExpectations(t *testing.T)

	// ScheduleCallback schedules the given callback after the given delay in workflow time (not wall clock).
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

type testWorkflow struct {
	instance      core.WorkflowInstance
	history       []history.Event
	pendingEvents []history.Event
}

type options struct {
	TestTimeout time.Duration
}

type workflowTester struct {
	options *options

	// Workflow under test
	wf  workflow.Workflow
	wfi core.WorkflowInstance

	// Workflows
	testWorkflows []*testWorkflow

	workflowFinished bool
	workflowResult   payload.Payload
	workflowErr      string

	registry *workflow.Registry

	ma               *mock.Mock
	mockedActivities map[string]bool

	mw              *mock.Mock
	mockedWorkflows map[string]bool

	workflowHistory []history.Event
	clock           *clock.Mock

	timers    []*testTimer
	callbacks chan func() *core.WorkflowEvent

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

		testWorkflows: make([]*testWorkflow, 0),

		ma:               &mock.Mock{},
		mockedActivities: make(map[string]bool),
		mw:               &mock.Mock{},

		workflowHistory: make([]history.Event, 0),
		clock:           clock,

		timers:    make([]*testTimer, 0),
		callbacks: make(chan func() *core.WorkflowEvent, 1024),
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
	// Start workflow under test
	wt.testWorkflows = append(wt.testWorkflows, &testWorkflow{
		instance:      wt.wfi,
		pendingEvents: []history.Event{wt.getInitialEvent(wt.wf, args)},
		history:       make([]history.Event, 0),
	})

	for !wt.workflowFinished {
		// Execute all workflows until no more events?
		gotNewEvents := false

		for _, tw := range wt.testWorkflows {
			if len(tw.pendingEvents) == 0 {
				// Nothing to process
				continue
			}

			// Get task
			t := getNextWorkflowTask(tw.instance, tw.history, tw.pendingEvents)
			tw.pendingEvents = tw.pendingEvents[:0]

			// Execute task
			e, err := workflow.NewExecutor(wt.registry, tw.instance, wt.clock)
			if err != nil {
				panic("could not create workflow executor" + err.Error())
			}

			executedEvents, workflowEvents, err := e.ExecuteTask(context.Background(), t)
			if err != nil {
				panic("Error while executing workflow" + err.Error())
			}

			e.Close()

			tw.history = append(tw.history, executedEvents...)

			for _, event := range executedEvents {
				log.Println("Event", event.Type)

				switch event.Type {
				case history.EventType_WorkflowExecutionFinished:
					// TODO: If sub-workflow, return result to calling workflow
					if !tw.instance.SubWorkflow() {
						wt.workflowFinished = true
						wt.workflowResult = event.Attributes.(*history.ExecutionCompletedAttributes).Result
						wt.workflowErr = event.Attributes.(*history.ExecutionCompletedAttributes).Error
					} else {
						// TODO: Send finished event to parent workflow
					}

				case history.EventType_ActivityScheduled:
					wt.scheduleActivity(tw.instance, event)
				}
			}

			for _, workflowEvent := range workflowEvents {
				log.Println("Workflow event", workflowEvent.HistoryEvent.Type)

				switch workflowEvent.HistoryEvent.Type {
				case history.EventType_SubWorkflowScheduled:
					// TODO: Mocked subworkflow, execute and add result message to parent workflow
					// TODO: Real subworkflow, start execution

				case history.EventType_TimerFired:
					wt.scheduleTimer(workflowEvent)
				}
			}
		}

		for !wt.workflowFinished && !gotNewEvents {
			// No new events left and the workflow isn't finished yet. Check for timers or callbacks
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

			if len(wt.timers) > 0 {
				// Take first timer and execute it
				sort.SliceStable(wt.timers, func(i, j int) bool {
					return wt.timers[i].At.Before(wt.timers[j].At)
				})

				t := wt.timers[0]
				wt.timers = wt.timers[1:]

				// Advance workflow clock to fire the timer
				log.Println("Advancing workflow clock to fire timer")
				wt.clock.Set(t.At)
				t.Callback()
			} else {
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
}

func (wt *workflowTester) sendEvent(wfi core.WorkflowInstance, event history.Event) {
	var w *testWorkflow
	for _, tw := range wt.testWorkflows {
		if tw.instance == wfi {
			w = tw
			break
		}
	}

	if w == nil {
		panic("TODO: Start new workflow instance")
	}

	w.pendingEvents = append(w.pendingEvents, event)
}

func (wt *workflowTester) SignalWorkflow(name string, value interface{}) {
	arg, err := converter.DefaultConverter.To(value)
	if err != nil {
		panic("Could not convert signal value to string" + err.Error())
	}

	wt.callbacks <- func() *core.WorkflowEvent {
		e := history.NewHistoryEvent(
			wt.clock.Now(),
			history.EventType_SignalReceived,
			-1,
			&history.SignalReceivedAttributes{
				Name: name,
				Arg:  arg,
			},
		)

		return &core.WorkflowEvent{
			WorkflowInstance: wt.wfi,
			HistoryEvent:     e,
		}
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

		wt.callbacks <- func() *core.WorkflowEvent {
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

			return &core.WorkflowEvent{
				WorkflowInstance: wfi,
				HistoryEvent:     ne,
			}
		}
	}()
}

func (wt *workflowTester) scheduleTimer(event core.WorkflowEvent) {
	e := event.HistoryEvent.Attributes.(*history.TimerFiredAttributes)

	wt.timers = append(wt.timers, &testTimer{
		At: e.At,
		Callback: func() {
			wt.callbacks <- func() *core.WorkflowEvent {
				return &event
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

func (wt *workflowTester) getInitialEvent(wf workflow.Workflow, args []interface{}) history.Event {
	name := fn.Name(wf)

	inputs, err := margs.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		panic(err)
	}

	return history.NewHistoryEvent(
		wt.clock.Now(),
		history.EventType_WorkflowExecutionStarted,
		-1,
		&history.ExecutionStartedAttributes{
			Name:   name,
			Inputs: inputs,
		},
	)
}

func getNextWorkflowTask(wfi core.WorkflowInstance, history []history.Event, newEvents []history.Event) *task.Workflow {
	return &task.Workflow{
		WorkflowInstance: wfi,
		History:          history,
		NewEvents:        newEvents,
	}
}
