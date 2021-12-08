package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/stretchr/testify/require"
)

func Test_ExecuteWorkflow(t *testing.T) {
	r := NewRegistry()

	var workflowHits int

	Workflow1 := func(ctx Context) error {
		workflowHits++

		return nil
	}

	r.RegisterWorkflow("w1", Workflow1)

	e := &executor{
		registry: r,
		workflow: NewWorkflow(Workflow1),
	}

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

	require.Equal(t, 1, workflowHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflow.context.commands, 1)
}

func Test_ReplayWorkflowWithActivityResult(t *testing.T) {
	r := NewRegistry()

	var workflowHit int

	Workflow1 := func(ctx Context) error {
		workflowHit++

		f1, err := ctx.ExecuteActivity("a1")
		if err != nil {
			panic("error executing activity 1")
		}

		_, err = f1.Get()
		if err != nil {
			panic("error getting activity 1 result")
		}

		workflowHit++

		return nil
	}
	Activity1 := func(ctx Context) (int, error) {
		fmt.Println("Entering Activity1")

		return 42, nil
	}

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)

	e := &executor{
		registry: r,
		workflow: NewWorkflow(Workflow1),
	}

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
				1,
				&history.ActivityScheduledAttributes{
					Name:    "a1",
					Version: "",
					Inputs:  [][]byte{},
				},
			),
			history.NewHistoryEvent(
				history.HistoryEventType_ActivityCompleted,
				1,
				&history.ActivityCompletedAttributes{
					ScheduleID: 0,
					Result:     "world",
				},
			),
		},
	})

	require.Equal(t, 2, workflowHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflow.context.commands, 1)
}

func Test_ExecuteWorkflowWithActivityCommand(t *testing.T) {
	r := NewRegistry()

	var workflowHits int

	Workflow1 := func(ctx Context) error {
		workflowHits++

		f1, err := ctx.ExecuteActivity("a1")
		if err != nil {
			panic("error executing activity 1")
		}

		_, err = f1.Get()
		if err != nil {
			panic("error getting activity 1 result")
		}

		workflowHits++

		return nil
	}
	Activity1 := func(ctx Context) (int, error) {
		fmt.Println("Entering Activity1")

		return 42, nil
	}

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)

	e := &executor{
		registry: r,
		workflow: NewWorkflow(Workflow1),
	}

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

	require.Equal(t, 1, workflowHits)

	require.Len(t, e.workflow.context.commands, 1)
	require.Equal(t, command.Command{
		ID:   0,
		Type: command.CommandType_ScheduleActivityTask,
		Attr: command.ScheduleActivityTaskCommandAttr{
			Name:    "a1",
			Version: "",
			Input:   "",
		},
	}, e.workflow.context.commands[0])
}
