package workflow

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func newExecutor(r *Registry, i *core.WorkflowInstance) *executor {
	s := workflowstate.NewWorkflowState(i, clock.New())
	wfCtx, cancel := sync.WithCancel(workflowstate.WithWorkflowState(sync.Background(), s))

	return &executor{
		registry:          r,
		workflow:          NewWorkflow(reflect.ValueOf(workflow1)),
		workflowState:     s,
		workflowCtx:       wfCtx,
		workflowCtxCancel: cancel,
		logger:            log.Default(),
		clock:             clock.New(),
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
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   "workflow1",
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 1, workflowHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
}

var workflowActivityHit int

func workflowWithActivity(ctx sync.Context) error {
	workflowActivityHit++

	f1 := wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42)

	_, err := f1.Get(ctx)
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
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithActivity",
					Inputs: []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_ActivityScheduled,
				&history.ActivityScheduledAttributes{
					Name:   "activity1",
					Inputs: []payload.Payload{inputs},
				},
				history.ScheduleEventID(1),
			),
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_ActivityCompleted,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
				history.ScheduleEventID(1),
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 2, workflowActivityHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
}

func Test_ExecuteWorkflowWithActivityCommand(t *testing.T) {
	r := NewRegistry()

	workflowActivityHit = 0

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
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
	require.Len(t, e.workflowState.Commands(), 1)

	inputs, _ := converter.DefaultConverter.To(42)
	require.Equal(t, command.Command{
		ID:    1,
		State: command.CommandState_Committed,
		Type:  command.CommandType_ScheduleActivityTask,
		Attr: &command.ScheduleActivityTaskCommandAttr{
			Name:   "activity1",
			Inputs: []payload.Payload{inputs},
		},
	}, *e.workflowState.Commands()[0])
}

var workflowTimerHits int

func workflowWithTimer(ctx sync.Context) error {
	workflowTimerHits++

	if _, err := wf.ScheduleTimer(ctx, time.Millisecond*5).Get(ctx); err != nil {
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
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
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
	require.Len(t, e.workflowState.Commands(), 1)

	require.Equal(t, 1, e.workflowState.Commands()[0].ID)
	require.Equal(t, command.CommandType_ScheduleTimer, e.workflowState.Commands()[0].Type)
}

var workflowWithSelectorHits int

func workflowWithSelector(ctx sync.Context) error {
	workflowWithSelectorHits++

	f1 := wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42)
	t := wf.ScheduleTimer(ctx, time.Millisecond*2)

	sync.Select(
		ctx,
		sync.Await[int](f1, func(ctx sync.Context, f sync.Future[int]) {
			workflowWithSelectorHits++
		}),
		sync.Await[struct{}](t, func(ctx sync.Context, _ sync.Future[struct{}]) {
			workflowWithSelectorHits++
		}),
	)

	workflowWithSelectorHits++

	return nil
}

func Test_ExecuteWorkflowWithSelector(t *testing.T) {
	r := NewRegistry()

	r.RegisterWorkflow(workflowWithSelector)
	r.RegisterActivity(activity1)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
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
	require.Len(t, e.workflowState.Commands(), 2)

	require.Equal(t, command.CommandType_ScheduleTimer, e.workflowState.Commands()[0].Type)
	require.Equal(t, command.CommandType_ScheduleActivityTask, e.workflowState.Commands()[1].Type)
}

func Test_ExecuteNewEvents(t *testing.T) {
	r := NewRegistry()

	workflowActivityHit = 0

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	inputs, _ := converter.DefaultConverter.To(42)
	result, _ := converter.DefaultConverter.To(42)

	oldTask := &task.Workflow{
		ID:               "oldtaskid",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History:          []history.Event{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithActivity",
					Inputs: []payload.Payload{inputs},
				},
			),
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_ActivityScheduled,
				&history.ActivityScheduledAttributes{
					Name:   "activity1",
					Inputs: []payload.Payload{inputs},
				},
				history.ScheduleEventID(1),
			),
		},
	}

	e := newExecutor(r, oldTask.WorkflowInstance)

	taskResult, err := e.ExecuteTask(context.Background(), oldTask)

	require.NoError(t, err)
	require.Equal(t, 1, workflowActivityHit)
	require.False(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 0)

	h := []history.Event{}
	h = append(h, oldTask.NewEvents...)
	h = append(h, taskResult.NewEvents...)

	newTask := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: oldTask.WorkflowInstance,
		History:          h,
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_ActivityCompleted,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
				history.ScheduleEventID(1),
			),
		},
		Kind: task.Continuation,
	}

	// Execute the workflow again with the activity completed event
	_, err = e.ExecuteTask(context.Background(), newTask)

	require.NoError(t, err)
	require.Equal(t, 2, workflowActivityHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
}

var workflowSignalHits int

func workflowWithSignal1(ctx sync.Context) error {
	c := wf.NewSignalChannel[string](ctx, "signal1")
	c.Receive(ctx)

	workflowSignalHits++

	return nil
}

func Test_ExecuteWorkflowWithSignal(t *testing.T) {
	r := NewRegistry()

	r.RegisterWorkflow(workflowWithSignal1)

	s, err := converter.DefaultConverter.To("")
	require.NoError(t, err)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.Event{
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   "workflowWithSignal1",
					Inputs: []payload.Payload{},
				},
			),
			history.NewHistoryEvent(
				time.Now(),
				history.EventType_SignalReceived,
				&history.SignalReceivedAttributes{
					Name: "signal1",
					Arg:  s,
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance)

	_, err = e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 1, workflowSignalHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
}
