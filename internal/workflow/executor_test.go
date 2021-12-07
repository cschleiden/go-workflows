package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
)

func Test_ExecuteWorkflow(t *testing.T) {
	r := NewRegistry()

	var workflowEntered int

	Workflow1 := func(ctx Context) error {
		fmt.Println("Entering Workflow1")
		workflowEntered++

		return nil
	}

	r.RegisterWorkflow("w1", Workflow1)

	e := NewExecutor(r)

	e.ExecuteWorkflowTask(context.Background(), tasks.WorkflowTask{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  [][]byte{},
				},
			),
		},
	})

	if workflowEntered != 1 {
		t.Fail()
	}

	// TODO: Assert completeness
}

func Test_ExecuteWorkflowWithActivity(t *testing.T) {
	r := NewRegistry()

	var workflowHit int

	Workflow1 := func(ctx Context) error {
		fmt.Println("Entering Workflow1")
		workflowHit++
		// fmt.Println("\tIsReplaying:", ctx.IsReplaying())

		f1, err := ctx.ExecuteActivity("a1")
		if err != nil {
			panic("error executing activity 1")
		}

		r1, err := f1.Get()
		if err != nil {
			panic("error getting activity 1 result")
		}
		fmt.Println("R1 result:", r1)

		fmt.Println("Leaving Workflow1")

		workflowHit++

		return nil
	}
	Activity1 := func(ctx Context) (int, error) {
		fmt.Println("Entering Activity1")

		return 42, nil
	}

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)

	e := NewExecutor(r)

	e.ExecuteWorkflowTask(context.Background(), tasks.WorkflowTask{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  [][]byte{},
				},
			),
			history.NewHistoryEvent(
				history.HistoryEventType_ActivityScheduled,
				-1,
				&history.ActivityScheduledAttributes{
					Name:    "a1",
					Version: "",
					Inputs:  [][]byte{},
				},
			),
			history.NewHistoryEvent(
				history.HistoryEventType_ActivityCompleted,
				-1,
				&history.ActivityCompletedAttributes{
					ScheduleID: 0,
					Result:     "world",
				},
			),
		},
	})

	if workflowHit != 2 {
		t.Fail()
	}

	// TODO: Assert completeness
}
