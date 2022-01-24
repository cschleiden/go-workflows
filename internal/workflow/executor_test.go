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

func newExecutor(r *Registry, i core.WorkflowInstance) *executor {
	state := newWorkflowState(i)
	wfCtx, cancel := sync.WithCancel(withWfState(sync.Background(), state))

	return &executor{
		registry:          r,
		workflow:          NewWorkflow(reflect.ValueOf(workflow1)),
		workflowState:     state,
		workflowCtx:       wfCtx,
		workflowCtxCancel: cancel,
		logger:            log.Default(),
	}
}

func activity1(ctx context.Context, r int) (int, error) {
	fmt.Println("Entering Activity1")

	return r, nil
}

var workflowHits int

func workflow1(ctx sync.Context) error {
	workflowHits++

	return nil
}

func Test_ExecuteWorkflow(t *testing.T) {
	r := NewRegistry()

	r.RegisterWorkflow(workflow1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflow1",
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	_, _, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 1, workflowHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 1)
}

var workflowActivityHit int

func workflowWithActivity(ctx sync.Context) error {
	workflowActivityHit++

	f1 := ExecuteActivity(ctx, DefaultActivityOptions, activity1, 42)

	var r int
	err := f1.Get(ctx, &r)
	if err != nil {
		panic("error getting activity 1 result")
	}

	workflowActivityHit++

	return nil
}

func Test_ReplayWorkflowWithActivityResult(t *testing.T) {
	r := NewRegistry()

	workflowActivityHit = 0

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	inputs, _ := converter.DefaultConverter.To(42)
	result, _ := converter.DefaultConverter.To(42)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithActivity",
					Inputs: []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				history.EventType_ActivityScheduled,
				0,
				&history.ActivityScheduledAttributes{
					Name:   "activity1",
					Inputs: []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				history.EventType_ActivityCompleted,
				0,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	_, _, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 2, workflowActivityHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 1)
}

func Test_ExecuteWorkflowWithActivityCommand(t *testing.T) {
	r := NewRegistry()

	workflowActivityHit = 0

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithActivity",
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	e.ExecuteTask(context.Background(), task)

	require.Equal(t, 1, workflowActivityHit)
	require.Len(t, e.workflowState.commands, 1)

	inputs, _ := converter.DefaultConverter.To(42)
	require.Equal(t, command.Command{
		ID:   0,
		Type: command.CommandType_ScheduleActivityTask,
		Attr: &command.ScheduleActivityTaskCommandAttr{
			Name:   "activity1",
			Inputs: []payload.Payload{inputs},
		},
	}, e.workflowState.commands[0])
}

var workflowTimerHits int

func workflowWithTimer(ctx sync.Context) error {
	workflowTimerHits++

	var r bool
	if err := ScheduleTimer(ctx, time.Millisecond*5).Get(ctx, &r); err != nil {
		panic("error getting timer future")
	}

	workflowTimerHits++

	return nil
}

func Test_ExecuteWorkflowWithTimer(t *testing.T) {
	r := NewRegistry()

	workflowTimerHits = 0

	r.RegisterWorkflow(workflowWithTimer)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithTimer",
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	e.ExecuteTask(context.Background(), task)

	require.Equal(t, 1, workflowTimerHits)
	require.Len(t, e.workflowState.commands, 1)

	require.Equal(t, 0, e.workflowState.commands[0].ID)
	require.Equal(t, command.CommandType_ScheduleTimer, e.workflowState.commands[0].Type)
}

var workflowWithSelectorHits int

func workflowWithSelector(ctx sync.Context) error {
	workflowWithSelectorHits++

	f1 := ExecuteActivity(ctx, DefaultActivityOptions, activity1, 42)
	t := ScheduleTimer(ctx, time.Millisecond*2)

	s := sync.NewSelector()
	s.AddFuture(f1, func(ctx sync.Context, f sync.Future) {
		workflowWithSelectorHits++
	})

	s.AddFuture(t, func(ctx sync.Context, t sync.Future) {
		workflowWithSelectorHits++
	})

	s.Select(ctx)

	workflowWithSelectorHits++

	return nil
}

func Test_ExecuteWorkflowWithSelector(t *testing.T) {
	r := NewRegistry()

	r.RegisterWorkflow(workflowWithSelector)
	r.RegisterActivity(activity1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithSelector",
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	e.ExecuteTask(context.Background(), task)

	require.Equal(t, 1, workflowWithSelectorHits)
	require.Len(t, e.workflowState.commands, 2)

	require.Equal(t, command.CommandType_ScheduleActivityTask, e.workflowState.commands[0].Type)
	require.Equal(t, command.CommandType_ScheduleTimer, e.workflowState.commands[1].Type)
}

func Test_ExecuteNewEvents(t *testing.T) {
	r := NewRegistry()

	workflowActivityHit = 0

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	inputs, _ := converter.DefaultConverter.To(42)
	result, _ := converter.DefaultConverter.To(42)

	oldTask := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History:          []history.Event{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithActivity",
					Inputs: []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				history.EventType_ActivityScheduled,
				0,
				&history.ActivityScheduledAttributes{
					Name:   "activity1",
					Inputs: []payload.Payload{inputs},
				},
			),
		},
	}

	e := newExecutor(r, oldTask.WorkflowInstance)

	newEvents, _, err := e.ExecuteTask(context.Background(), oldTask)

	require.NoError(t, err)
	require.Equal(t, 1, workflowActivityHit)
	require.False(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 0)

	h := []history.Event{}
	h = append(h, oldTask.NewEvents...)
	h = append(h, newEvents...)

	newTask := &task.Workflow{
		WorkflowInstance: oldTask.WorkflowInstance,
		History:          h,
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				history.EventType_ActivityCompleted,
				0,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
			),
		},
		Kind: task.Continuation,
	}

	// Execute the workflow again with the activity completed event
	_, _, err = e.ExecuteTask(context.Background(), newTask)

	require.NoError(t, err)
	require.Equal(t, 2, workflowActivityHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 1)
}

var workflowSignalHits int

func workflowWithSignal1(ctx sync.Context) error {

	c := NewSignalChannel(ctx, "signal1")
	c.Receive(ctx, nil)

	workflowSignalHits++

	return nil
}

func Test_ExecuteWorkflowWithSignal(t *testing.T) {
	r := NewRegistry()

	r.RegisterWorkflow(workflowWithSignal1)

	task := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				history.EventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithSignal1",
					Inputs: []payload.Payload{},
				},
			),
			history.NewHistoryEvent(
				history.EventType_SignalReceived,
				-1,
				&history.SignalReceivedAttributes{
					Name: "signal1",
					Arg:  payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	_, _, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 1, workflowSignalHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.commands, 1)
}
