package workflow

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/stretchr/testify/require"
)

func Test_ExecuteWorkflow(t *testing.T) {
	r := NewRegistry()

	var workflowHits int

	Workflow1 := func(ctx sync.Context) error {
		workflowHits++

		return nil
	}

	r.RegisterWorkflow("w1", Workflow1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  []payload.Payload{},
				},
			),
		},
	}

	e := &executor{
		registry:      r,
		task:          task,
		workflow:      NewWorkflow(reflect.ValueOf(Workflow1)),
		workflowState: newWorkflowState(),
		logger:        log.Default(),
	}

	e.ExecuteWorkflowTask(context.Background())

	require.Equal(t, 1, workflowHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 1)
}

func Test_ReplayWorkflowWithActivityResult(t *testing.T) {
	r := NewRegistry()

	var workflowHit int

	Workflow1 := func(ctx sync.Context) error {
		workflowHit++

		f1, err := ExecuteActivity(ctx, "a1", 42)
		if err != nil {
			panic("error executing activity 1")
		}

		var r int
		err = f1.Get(ctx, &r)
		if err != nil {
			panic("error getting activity 1 result")
		}

		workflowHit++

		return nil
	}
	Activity1 := func(ctx context.Context, r int) (int, error) {
		fmt.Println("Entering Activity1")

		return r, nil
	}

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)

	inputs, _ := converter.DefaultConverter.To(42)
	result, _ := converter.DefaultConverter.To(42)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				history.HistoryEventType_ActivityScheduled,
				0,
				&history.ActivityScheduledAttributes{
					Name:    "a1",
					Version: "",
					Inputs:  []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				history.HistoryEventType_ActivityCompleted,
				0,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
			),
		},
	}

	e := &executor{
		registry:      r,
		task:          task,
		workflow:      NewWorkflow(reflect.ValueOf(Workflow1)),
		workflowState: newWorkflowState(),
		logger:        log.Default(),
	}

	e.ExecuteWorkflowTask(context.Background())

	require.Equal(t, 2, workflowHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 1)
}

func Test_ExecuteWorkflowWithActivityCommand(t *testing.T) {
	r := NewRegistry()

	var workflowHits int

	Workflow1 := func(ctx sync.Context) error {
		workflowHits++

		f1, err := ExecuteActivity(ctx, "a1", 42)
		if err != nil {
			panic("error executing activity 1")
		}

		var r int
		err = f1.Get(ctx, &r)
		if err != nil {
			panic("error getting activity 1 result")
		}

		workflowHits++

		return nil
	}
	Activity1 := func(ctx context.Context, r int) (int, error) {
		fmt.Println("Entering Activity1")

		return r, nil
	}

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  []payload.Payload{},
				},
			),
		},
	}

	e := &executor{
		registry:      r,
		task:          task,
		workflow:      NewWorkflow(reflect.ValueOf(Workflow1)),
		workflowState: newWorkflowState(),
		logger:        log.Default(),
	}

	e.ExecuteWorkflowTask(context.Background())

	require.Equal(t, 1, workflowHits)

	require.Len(t, e.workflowState.commands, 1)

	inputs, _ := converter.DefaultConverter.To(42)
	require.Equal(t, command.Command{
		ID:   0,
		Type: command.CommandType_ScheduleActivityTask,
		Attr: &command.ScheduleActivityTaskCommandAttr{
			Name:    "a1",
			Version: "",
			Inputs:  []payload.Payload{inputs},
		},
	}, e.workflowState.commands[0])
}

func Test_ExecuteWorkflowWithTimer(t *testing.T) {
	r := NewRegistry()

	var workflowHits int

	Workflow1 := func(ctx sync.Context) error {
		workflowHits++

		t, err := ScheduleTimer(ctx, time.Millisecond*5)
		if err != nil {
			panic("error executing activity 1")
		}

		var r bool
		err = t.Get(ctx, &r)
		if err != nil {
			panic("error getting timer future")
		}

		workflowHits++

		return nil
	}

	r.RegisterWorkflow("w1", Workflow1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  []payload.Payload{},
				},
			),
		},
	}

	e := &executor{
		registry:      r,
		task:          task,
		workflow:      NewWorkflow(reflect.ValueOf(Workflow1)),
		workflowState: newWorkflowState(),
		logger:        log.Default(),
	}

	e.ExecuteWorkflowTask(context.Background())

	require.Equal(t, 1, workflowHits)
	require.Len(t, e.workflowState.commands, 1)

	require.Equal(t, 0, e.workflowState.commands[0].ID)
	require.Equal(t, command.CommandType_ScheduleTimer, e.workflowState.commands[0].Type)
}

func Test_ExecuteWorkflowWithSelector(t *testing.T) {
	r := NewRegistry()

	var workflowHits int

	Workflow1 := func(ctx sync.Context) error {
		workflowHits++

		s := sync.NewSelector()

		f1, err := ExecuteActivity(ctx, "a1", 42)
		if err != nil {
			panic("error executing activity 1")
		}
		t, err := ScheduleTimer(ctx, time.Millisecond*2)
		if err != nil {
			panic("error executing activity 1")
		}

		s.AddFuture(f1, func(ctx sync.Context, f sync.Future) {
			workflowHits++
		})

		s.AddFuture(t, func(ctx sync.Context, t sync.Future) {
			workflowHits++
		})

		s.Select(ctx)

		workflowHits++

		return nil
	}
	Activity1 := func(ctx context.Context, r int) (int, error) {
		fmt.Println("Entering Activity1")

		return r, nil
	}

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "w1",
					Version: "",
					Inputs:  []payload.Payload{},
				},
			),
		},
	}

	e := &executor{
		registry:      r,
		task:          task,
		workflow:      NewWorkflow(reflect.ValueOf(Workflow1)),
		workflowState: newWorkflowState(),
		logger:        log.Default(),
	}

	e.ExecuteWorkflowTask(context.Background())

	require.Equal(t, 1, workflowHits)
	require.Len(t, e.workflowState.commands, 2)

	require.Equal(t, command.CommandType_ScheduleActivityTask, e.workflowState.commands[0].Type)
	require.Equal(t, command.CommandType_ScheduleTimer, e.workflowState.commands[1].Type)
}
