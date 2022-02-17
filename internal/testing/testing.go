package testing

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-dt/internal/activity"
	margs "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type WorkflowTester interface {
	Execute(args ...interface{})

	Registry() *workflow.Registry

	OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call

	OnSubWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call

	// OnSignal() // TODO: Allow waiting

	// SignalWorkflow( /*TODO*/ )

	WorkflowFinished() bool

	WorkflowResult(vtpr interface{}, err *string)

	AssertExpectations(t *testing.T)
}

type workflowTester struct {
	// Workflow under test
	wf  workflow.Workflow
	wfi core.WorkflowInstance

	e workflow.WorkflowExecutor

	workflowFinished bool
	workflowResult   payload.Payload
	workflowErr      string

	registry         *workflow.Registry
	ma               *mock.Mock
	mockedActivities map[string]bool
	mw               *mock.Mock

	clock *clock.Mock
}

func NewWorkflowTester(wf workflow.Workflow) WorkflowTester {
	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	registry := workflow.NewRegistry()
	e, err := workflow.NewExecutor(registry, wfi)
	if err != nil {
		panic("could not create workflow executor" + err.Error())
	}

	wt := &workflowTester{
		wf:       wf,
		wfi:      wfi,
		e:        e,
		registry: registry,

		ma:               &mock.Mock{},
		mockedActivities: make(map[string]bool),
		mw:               &mock.Mock{},

		clock: clock.NewMock(),
	}

	// Always register the workflow under test
	wt.registry.RegisterWorkflow(wf)

	return wt
}

func (wt *workflowTester) Registry() *workflow.Registry {
	return wt.registry
}

func (wt *workflowTester) OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call {
	// Register activity so that we can correctly identify its arguments later
	wt.registry.RegisterActivity(activity)

	name := fn.Name(activity)
	call := wt.ma.On(name, args...)
	wt.mockedActivities[name] = true
	return call
}

func (wt *workflowTester) OnSubWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call {
	wt.registry.RegisterWorkflow(workflow)

	name := fn.Name(workflow)
	return wt.mw.On(name, args...)
}

func (wt *workflowTester) Execute(args ...interface{}) {
	task := wt.getInitialWorkflowTask(wt.wfi, wt.wf, args...)

	for !wt.workflowFinished {
		// TODO: Handle workflow events
		executedEvents, _ /*workflowEvents*/, err := wt.e.ExecuteTask(context.Background(), task)
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
				newEvents = append(newEvents, wt.getActivityResultEvent(task.WorkflowInstance, event))

			case history.EventType_SubWorkflowScheduled:
				newEvents = append(newEvents, wt.getSubWorkflowResultEvent(event))

			case history.EventType_TimerScheduled:
				newEvents = append(newEvents, wt.getTimerResultEvent(event))
			}

			log.Println(event.Type.String())

			// TODO: Timers
			// TODO: SubWorkflows
			// TODO: Signals?
		}

		// TODO: How does this work with timers?
		if len(newEvents) == 0 && !wt.workflowFinished {
			panic("No new events generated during workflow execution, workflow blocked?")
		}

		task = getNextWorkflowTask(wt.wfi, executedEvents, newEvents)
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

func (wt *workflowTester) getActivityResultEvent(wfi core.WorkflowInstance, event history.Event) history.Event {
	e := event.Attributes.(*history.ActivityScheduledAttributes)

	var activityErr error
	var activityResult interface{}

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
			activityResult = results.Get(0)
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
		// Execute real activity
		executor := activity.NewExecutor(wt.registry)
		activityResult, activityErr = executor.ExecuteActivity(context.Background(), &task.Activity{
			ID:               uuid.NewString(),
			WorkflowInstance: wfi,
			Event:            event,
		})
	}

	if activityErr != nil {
		return history.NewHistoryEvent(
			history.EventType_ActivityFailed,
			event.EventID,
			&history.ActivityFailedAttributes{
				Reason: activityErr.Error(),
			},
		)
	} else {
		result, err := converter.DefaultConverter.To(activityResult)
		if err != nil {
			panic("Could not convert result for activity " + e.Name + ": " + err.Error())
		}

		return history.NewHistoryEvent(
			history.EventType_ActivityCompleted,
			event.EventID,
			&history.ActivityCompletedAttributes{
				Result: result,
			},
		)
	}
}

func (wt *workflowTester) getTimerResultEvent(event history.Event) history.Event {
	e := event.Attributes.(*history.TimerScheduledAttributes)

	// TOOD: Implement support for timers
	return history.NewFutureHistoryEvent(
		history.EventType_TimerFired,
		event.EventID,
		&history.TimerFiredAttributes{},
		e.At,
	)
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
		history.EventType_WorkflowExecutionStarted,
		-1,
		&history.ExecutionStartedAttributes{
			Name:   name,
			Inputs: inputs,
		},
	)

	event.Timestamp = wt.clock.Now()

	return &task.Workflow{
		WorkflowInstance: wfi,
		History:          []history.Event{event},
	}
}

func getNextWorkflowTask(wfi core.WorkflowInstance, history []history.Event, newEvents []history.Event) *task.Workflow {
	return &task.Workflow{
		WorkflowInstance: wfi,
		History:          history,
		NewEvents:        newEvents,
	}
}
