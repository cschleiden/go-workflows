package testing

import (
	"context"
	"testing"

	margs "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type WorkflowTester interface {
	Execute(args ...interface{}) error

	OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call
	// OnWorkflow(workflow workflow.Workflow, args ...interface{}) *mock.Call

	// OnSignal() // TODO: Allow waiting

	// SignalWorkflow( /*TODO*/ )

	WorkflowFinished() bool

	AssertExpectations(t *testing.T)
}

type workflowTester struct {
	wf               workflow.Workflow
	workflowFinished bool
	result           interface{}
	registry         *workflow.Registry
	ma               *mock.Mock
}

func NewWorkflowTester(wf workflow.Workflow) WorkflowTester {
	wt := &workflowTester{
		wf:       wf,
		registry: workflow.NewRegistry(),
		ma:       &mock.Mock{},
	}

	// Always register the workflow under test
	wt.registry.RegisterWorkflow(wf)

	return wt
}

func (wt *workflowTester) OnActivity(activity workflow.Activity, args ...interface{}) *mock.Call {
	name := fn.Name(activity)
	return wt.ma.On(name, args...)
}

func (wt *workflowTester) Execute(args ...interface{}) error {
	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())

	e, err := workflow.NewExecutor(wt.registry, wfi)
	if err != nil {
		// TODO: t.Fail instead here? Or just return?
		panic(err)
	}

	task := getInitialWorkflowTask(wfi, wt.wf, args...)

	for !wt.workflowFinished {
		executedEvents, _ /*workflowEvents*/, err := e.ExecuteTask(context.Background(), task)
		if err != nil {
			panic(err)
			//return err
		}

		// Process events for next task
		newEvents := make([]history.Event, 0)

		for _, event := range executedEvents {
			switch event.Type {
			case history.EventType_WorkflowExecutionFinished:
				wt.workflowFinished = true
				wt.result = event.Attributes.(*history.ExecutionCompletedAttributes).Result

			case history.EventType_ActivityScheduled:
				e := event.Attributes.(*history.ActivityScheduledAttributes)
				result := wt.ma.MethodCalled(e.Name).Get(0) // TODO: Inputs
				// TODO: Failures
				r, _ := converter.DefaultConverter.To(result)
				newEvents = append(newEvents, history.NewHistoryEvent(
					history.EventType_ActivityCompleted,
					event.EventID,
					&history.ActivityCompletedAttributes{
						Result: r,
					},
				))
			}
		}

		task = getNextWorkflowTask(wfi, executedEvents, newEvents)
	}

	// TODO: Get and return workflow result
	return nil
}

func (wt *workflowTester) WorkflowFinished() bool {
	return wt.workflowFinished
}

func (wt *workflowTester) AssertExpectations(t *testing.T) {
	t.Helper()

	wt.ma.AssertExpectations(t)
}

func getInitialWorkflowTask(wfi core.WorkflowInstance, wf workflow.Workflow, args ...interface{}) *task.Workflow {
	name := fn.Name(wf)

	inputs, err := margs.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		panic(err)
	}

	return &task.Workflow{
		WorkflowInstance: wfi,
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   name,
					Inputs: inputs,
				},
			),
		},
	}
}

func getNextWorkflowTask(wfi core.WorkflowInstance, history []history.Event, newEvents []history.Event) *task.Workflow {
	return &task.Workflow{
		WorkflowInstance: wfi,
		History:          history,
		NewEvents:        newEvents,
	}
}
