package workflow

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/logger"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/task"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

type testHistoryProvider struct {
	history []history.Event
}

func (t *testHistoryProvider) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]history.Event, error) {
	return t.history, nil
}

func newExecutor(r *Registry, i *core.WorkflowInstance, workflow interface{}, historyProvider WorkflowHistoryProvider) *executor {
	logger := logger.NewDefaultLogger()
	tracer := trace.NewNoopTracerProvider().Tracer("test")

	e, err := NewExecutor(logger, tracer, r, historyProvider, i, clock.New())
	if err != nil {
		panic(err)
	}

	return e.(*executor)
}

func activity1(ctx context.Context, r int) (int, error) {
	log.Println("Entering Activity1")

	return r, nil
}

func Test_ExecuteWorkflow(t *testing.T) {
	var workflowHits int
	workflow1 := func(ctx sync.Context) error {
		workflowHits++
		return nil
	}

	r := NewRegistry()

	r.RegisterWorkflow(workflow1)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				1,
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflow1),
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance, workflow1, &testHistoryProvider{})

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	require.Equal(t, 1, workflowHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
	require.IsType(t, &command.CompleteWorkflowCommand{}, e.workflowState.Commands()[0])
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
		Metadata:         &core.WorkflowMetadata{},
		LastSequenceID:   3,
	}

	e := newExecutor(r, task.WorkflowInstance, workflowWithActivity, &testHistoryProvider{[]history.Event{
		history.NewHistoryEvent(
			1,
			time.Now(),
			history.EventType_WorkflowExecutionStarted,
			&history.ExecutionStartedAttributes{
				Name:   fn.Name(workflowWithActivity),
				Inputs: []payload.Payload{},
			},
		),
		history.NewHistoryEvent(
			2,
			time.Now(),
			history.EventType_ActivityScheduled,
			&history.ActivityScheduledAttributes{
				Name:   "activity1",
				Inputs: []payload.Payload{inputs},
			},
			history.ScheduleEventID(1),
		),
		history.NewHistoryEvent(
			3,
			time.Now(),
			history.EventType_ActivityCompleted,
			&history.ActivityCompletedAttributes{
				Result: result,
			},
			history.ScheduleEventID(1),
		),
	}})

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 2, workflowActivityHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 2)
}

func pendingCommands(commands []command.Command) []command.Command {
	var pending []command.Command
	for _, c := range commands {
		if c.State() == command.CommandState_Pending {
			pending = append(pending, c)
		}
	}
	return pending
}

func Test_ExecuteWorkflowWithActivityCommand(t *testing.T) {
	r := NewRegistry()

	workflowActivityHit = 0

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				1,
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowWithActivity),
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance, workflowWithActivity, &testHistoryProvider{})

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 1, workflowActivityHit)
	require.Len(t, e.workflowState.Commands(), 1)

	inputs, _ := converter.DefaultConverter.To(42)
	require.IsType(t, &command.ScheduleActivityCommand{}, e.workflowState.Commands()[0])
	require.Equal(t, command.CommandState_Committed, e.workflowState.Commands()[0].State())
	require.Equal(t, "activity1", e.workflowState.Commands()[0].(*command.ScheduleActivityCommand).Name)
	require.Equal(t, []payload.Payload{inputs}, e.workflowState.Commands()[0].(*command.ScheduleActivityCommand).Inputs)
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
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				1,
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowWithTimer),
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance, workflowWithTimer, &testHistoryProvider{})

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 1, workflowTimerHits)
	require.Len(t, e.workflowState.Commands(), 1)

	require.Equal(t, int64(1), e.workflowState.Commands()[0].ID())
	require.IsType(t, &command.ScheduleTimerCommand{}, e.workflowState.Commands()[0])
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
	workflowWithSelectorHits = 0

	r := NewRegistry()

	r.RegisterWorkflow(workflowWithSelector)
	r.RegisterActivity(activity1)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				1,
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowWithSelector),
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance, workflowWithSelector, &testHistoryProvider{})

	_, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 1, workflowWithSelectorHits)
	require.Len(t, e.workflowState.Commands(), 2)

	require.IsType(t, &command.ScheduleActivityCommand{}, e.workflowState.Commands()[0])
	require.IsType(t, &command.ScheduleTimerCommand{}, e.workflowState.Commands()[1])
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
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowWithActivity),
					Inputs: []payload.Payload{},
				},
			),
			history.NewPendingEvent(
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

	e := newExecutor(r, oldTask.WorkflowInstance, workflowWithActivity, &testHistoryProvider{[]history.Event{}})

	taskResult, err := e.ExecuteTask(context.Background(), oldTask)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 1, workflowActivityHit)
	require.False(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)

	h := []history.Event{}
	h = append(h, oldTask.NewEvents...)
	h = append(h, taskResult.Executed...)

	newTask := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: oldTask.WorkflowInstance,
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewHistoryEvent(
				1,
				time.Now(),
				history.EventType_ActivityCompleted,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
				history.ScheduleEventID(1),
			),
		},
		LastSequenceID: taskResult.Executed[len(taskResult.Executed)-1].SequenceID,
	}

	// Execute the workflow again with the activity completed event
	_, err = e.ExecuteTask(context.Background(), newTask)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 2, workflowActivityHit)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 2)
}

func Test_ExecuteWorkflowWithSignal(t *testing.T) {
	r := NewRegistry()

	var workflowSignalHits int

	workflowWithSignal := func(ctx sync.Context) error {
		c := wf.NewSignalChannel[string](ctx, "signal1")
		c.Receive(ctx)

		workflowSignalHits++

		return nil
	}

	r.RegisterWorkflow(workflowWithSignal)

	s, err := converter.DefaultConverter.To("")
	require.NoError(t, err)

	task := &task.Workflow{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowWithSignal),
					Inputs: []payload.Payload{},
				},
			),
			history.NewPendingEvent(
				time.Now(),
				history.EventType_SignalReceived,
				&history.SignalReceivedAttributes{
					Name: "signal1",
					Arg:  s,
				},
			),
		},
	}

	e := newExecutor(r, task.WorkflowInstance, workflowWithSignal, &testHistoryProvider{})

	_, err = e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Equal(t, 1, workflowSignalHits)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
}

func Test_CompletesWorkflowOnError(t *testing.T) {
	r := NewRegistry()

	workflowPanic := func(ctx sync.Context) error {
		panic("wf error")
	}

	r.RegisterWorkflow(workflowPanic)

	task1 := &task.Workflow{
		ID:               "taskid",
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowPanic),
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	historyProvider := &testHistoryProvider{[]history.Event{}}
	e := newExecutor(r, task1.WorkflowInstance, workflowPanic, historyProvider)

	r1, err := e.ExecuteTask(context.Background(), task1)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 1)
	require.Len(t, pendingCommands(e.workflowState.Commands()), 0)
	require.True(t, r1.Completed)
}

func Test_ScheduleSubWorkflow(t *testing.T) {
	r := NewRegistry()

	subworkflow := func(ctx wf.Context) error {
		return nil
	}

	workflow := func(ctx wf.Context) error {
		_, err := wf.CreateSubWorkflowInstance[any](ctx, wf.SubWorkflowOptions{
			InstanceID: "subworkflow",
		}, subworkflow).Get(ctx)

		return err
	}

	r.RegisterWorkflow(workflow)
	r.RegisterWorkflow(subworkflow)

	task := startWorkflowTask("instanceID", workflow)
	hp := &testHistoryProvider{}
	e := newExecutor(r, task.WorkflowInstance, workflow, hp)
	result, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Len(t, result.Executed, 3)
	require.Len(t, result.WorkflowEvents, 1)
	require.Equal(t, history.EventType_WorkflowExecutionStarted, result.WorkflowEvents[0].HistoryEvent.Type)
}

func Test_ScheduleSubWorkflow_Cancel(t *testing.T) {
	r := NewRegistry()

	subworkflow := func(ctx wf.Context) error {
		return nil
	}

	workflow := func(ctx wf.Context) error {
		swctx, cancel := wf.WithCancel(ctx)

		wf.CreateSubWorkflowInstance[any](swctx, wf.SubWorkflowOptions{
			InstanceID: "subworkflow",
		}, subworkflow)

		wf.Sleep(ctx, time.Millisecond)

		cancel()

		return nil
	}

	r.RegisterWorkflow(workflow)
	r.RegisterWorkflow(subworkflow)

	task := startWorkflowTask("instanceID", workflow)
	hp := &testHistoryProvider{}
	e := newExecutor(r, task.WorkflowInstance, workflow, hp)
	result, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, e.workflow.err)
	require.Len(t, result.Executed, 4)
	require.Len(t, result.TimerEvents, 1)
	require.Len(t, result.WorkflowEvents, 1)
	require.Equal(t, history.EventType_WorkflowExecutionStarted, result.WorkflowEvents[0].HistoryEvent.Type)

	subWorkflowInstance := result.WorkflowEvents[0].WorkflowInstance

	// Go past Sleep
	hp.history = append(hp.history, result.Executed...)
	result, err = e.ExecuteTask(context.Background(), continueTask("instanceID", []history.Event{
		result.TimerEvents[0],
	}, result.Executed[len(result.Executed)-1].SequenceID))
	require.NoError(t, err)
	require.Len(t, result.WorkflowEvents, 1, "Cancellation should have been requested")
	require.Equal(t, history.EventType_WorkflowExecutionCanceled, result.WorkflowEvents[0].HistoryEvent.Type)
	require.Equal(
		t,
		subWorkflowInstance,
		result.WorkflowEvents[0].WorkflowInstance)

	require.True(t, e.workflow.Completed())
	require.Len(t, e.workflowState.Commands(), 3)
}

func startWorkflowTask(instanceID string, workflow interface{}, workflowArgs ...interface{}) *task.Workflow {
	inputs, err := args.ArgsToInputs(converter.DefaultConverter, workflowArgs...)
	if err != nil {
		panic(err)
	}

	return &task.Workflow{
		ID:               uuid.NewString(),
		WorkflowInstance: core.NewWorkflowInstance(instanceID, "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents: []history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflow),
					Inputs: inputs,
				},
			),
		},
	}
}

func continueTask(instanceID string, newEvents []history.Event, lastSequenceID int64) *task.Workflow {
	return &task.Workflow{
		ID:               uuid.NewString(),
		WorkflowInstance: core.NewWorkflowInstance(instanceID, "executionID"),
		Metadata:         &core.WorkflowMetadata{},
		NewEvents:        newEvents,
		LastSequenceID:   lastSequenceID,
	}
}
