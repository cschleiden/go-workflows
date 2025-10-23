package executor

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/registry"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_InstanceExecutionDetails_SimpleWorkflow(t *testing.T) {
	r := registry.New()
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	hp := &testHistoryProvider{}
	e, err := newExecutor(r, i, hp)
	require.NoError(t, err)
	defer e.Close()

	var capturedLength int64
	simpleWorkflow := func(ctx sync.Context) error {
		info := wf.InstanceExecutionDetails(ctx)
		capturedLength = info.HistoryLength
		return nil
	}

	r.RegisterWorkflow(simpleWorkflow)

	task := startWorkflowTask(i.InstanceID, simpleWorkflow)
	result, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	// The workflow code runs when WorkflowExecutionStarted is processed (event index 1)
	// At that point: WorkflowTaskStarted (index 0, will be seq 1) and WorkflowExecutionStarted (index 1, will be seq 2)
	// So history length should be 2
	require.Equal(t, int64(2), capturedLength)
	require.Equal(t, int64(3), e.lastSequenceID)
	require.Equal(t, 3, len(result.Executed)) // WorkflowTaskStarted, WorkflowExecutionStarted, WorkflowExecutionFinished
}

func Test_InstanceExecutionDetails_WithActivity(t *testing.T) {
	r := registry.New()
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	hp := &testHistoryProvider{}
	e, err := newExecutor(r, i, hp)
	require.NoError(t, err)
	defer e.Close()

	var lengthBeforeActivity, lengthAfterActivity int64

	workflowWithActivity := func(ctx sync.Context) error {
		info := wf.InstanceExecutionDetails(ctx)
		lengthBeforeActivity = info.HistoryLength

		wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42).Get(ctx)

		info = wf.InstanceExecutionDetails(ctx)
		lengthAfterActivity = info.HistoryLength

		return nil
	}

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	inputs, _ := converter.DefaultConverter.To(42)
	result, _ := converter.DefaultConverter.To(42)

	task := &backend.WorkflowTask{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance(i.InstanceID, "executionID"),
		Metadata:         &metadata.WorkflowMetadata{},
		NewEvents: []*history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflowWithActivity),
					Inputs: []payload.Payload{},
				},
			),
		},
	}

	// Execute first task - schedules activity
	task1Result, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, int64(2), lengthBeforeActivity)

	// Setup history for replay
	hp.history = []*history.Event{
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
			history.EventType_WorkflowTaskStarted,
			&history.WorkflowTaskStartedAttributes{},
		),
		history.NewHistoryEvent(
			3,
			time.Now(),
			history.EventType_ActivityScheduled,
			&history.ActivityScheduledAttributes{
				Name:   "activity1",
				Inputs: []payload.Payload{inputs},
			},
			history.ScheduleEventID(1),
		),
	}

	// Execute second task with activity completion
	task2 := &backend.WorkflowTask{
		ID:               "taskID2",
		WorkflowInstance: core.NewWorkflowInstance(i.InstanceID, "executionID"),
		Metadata:         &metadata.WorkflowMetadata{},
		NewEvents: []*history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_ActivityCompleted,
				&history.ActivityCompletedAttributes{
					Result: result,
				},
				history.ScheduleEventID(1),
			),
		},
		LastSequenceID: task1Result.Executed[len(task1Result.Executed)-1].SequenceID,
	}

	_, err = e.ExecuteTask(context.Background(), task2)
	require.NoError(t, err)

	// History should have grown: WorkflowExecutionStarted, WorkflowTaskStarted, ActivityScheduled, WorkflowTaskStarted, ActivityCompleted
	require.Greater(t, lengthAfterActivity, lengthBeforeActivity)
	require.Equal(t, int64(5), lengthAfterActivity)
}

func Test_InstanceExecutionDetails_DuringReplay(t *testing.T) {
	r := registry.New()
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	hp := &testHistoryProvider{}
	e, err := newExecutor(r, i, hp)
	require.NoError(t, err)
	defer e.Close()

	var lengths []int64

	workflowWithActivity := func(ctx sync.Context) error {
		// Capture length at start
		info := wf.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42).Get(ctx)

		// Capture length after activity
		info = wf.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		return nil
	}

	r.RegisterWorkflow(workflowWithActivity)
	r.RegisterActivity(activity1)

	inputs, _ := converter.DefaultConverter.To(42)
	result, _ := converter.DefaultConverter.To(42)

	// Setup complete history for replay
	hp.history = []*history.Event{
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
			history.EventType_WorkflowTaskStarted,
			&history.WorkflowTaskStartedAttributes{},
		),
		history.NewHistoryEvent(
			3,
			time.Now(),
			history.EventType_ActivityScheduled,
			&history.ActivityScheduledAttributes{
				Name:   "activity1",
				Inputs: []payload.Payload{inputs},
			},
			history.ScheduleEventID(1),
		),
		history.NewHistoryEvent(
			4,
			time.Now(),
			history.EventType_WorkflowTaskStarted,
			&history.WorkflowTaskStartedAttributes{},
		),
		history.NewHistoryEvent(
			5,
			time.Now(),
			history.EventType_ActivityCompleted,
			&history.ActivityCompletedAttributes{
				Result: result,
			},
			history.ScheduleEventID(1),
		),
	}

	task := &backend.WorkflowTask{
		ID:               "taskID",
		WorkflowInstance: core.NewWorkflowInstance(i.InstanceID, "executionID"),
		Metadata:         &metadata.WorkflowMetadata{},
		LastSequenceID:   5,
	}

	_, err = e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	// During replay, the workflow replays up to sequenceID 5
	// First time workflow code runs: after replaying event 1 (WorkflowExecutionStarted), history length = 1
	// Second time: after replaying event 5 (ActivityCompleted), then new WorkflowTaskStarted is added
	require.Len(t, lengths, 2)
	require.Equal(t, int64(1), lengths[0]) // After replaying WorkflowExecutionStarted
	require.Equal(t, int64(5), lengths[1]) // After replaying all events including ActivityCompleted
}

func Test_InstanceExecutionDetails_Incremental(t *testing.T) {
	r := registry.New()
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	hp := &testHistoryProvider{}
	e, err := newExecutor(r, i, hp)
	require.NoError(t, err)
	defer e.Close()

	var lengths []int64

	workflowMultipleSteps := func(ctx sync.Context) error {
		info := wf.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		wf.Sleep(ctx, time.Millisecond)

		info = wf.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		wf.Sleep(ctx, time.Millisecond)

		info = wf.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		return nil
	}

	r.RegisterWorkflow(workflowMultipleSteps)

	// First execution - start workflow, schedule first timer
	task1 := startWorkflowTask(i.InstanceID, workflowMultipleSteps)
	result1, err := e.ExecuteTask(context.Background(), task1)
	require.NoError(t, err)

	// Second execution - first timer fires, schedule second timer
	hp.history = append(hp.history, result1.Executed...)
	task2 := continueTask(i.InstanceID, []*history.Event{
		history.NewPendingEvent(time.Now(), history.EventType_TimerFired, &history.TimerFiredAttributes{
			ScheduledAt: time.Now(),
			At:          time.Now().Add(time.Millisecond),
		}, history.ScheduleEventID(1)),
	}, result1.Executed[len(result1.Executed)-1].SequenceID)
	result2, err := e.ExecuteTask(context.Background(), task2)
	require.NoError(t, err)

	// Third execution - second timer fires, complete workflow
	hp.history = append(hp.history, result2.Executed...)
	task3 := continueTask(i.InstanceID, []*history.Event{
		history.NewPendingEvent(time.Now(), history.EventType_TimerFired, &history.TimerFiredAttributes{
			ScheduledAt: time.Now(),
			At:          time.Now().Add(time.Millisecond),
		}, history.ScheduleEventID(2)),
	}, result2.Executed[len(result2.Executed)-1].SequenceID)
	_, err = e.ExecuteTask(context.Background(), task3)
	require.NoError(t, err)

	// Verify history length increases at each step
	require.Len(t, lengths, 3)
	require.Equal(t, int64(2), lengths[0]) // Initial: WorkflowTaskStarted(1) + WorkflowExecutionStarted(2)
	// After first timer fires: previous 3 events + WorkflowTaskStarted(4) + TimerFired(5) = 5,
	// but we see it when WorkflowTaskStarted is being processed, so we see 4 + upcoming event = 5
	require.Greater(t, lengths[1], lengths[0], "History should grow after first timer")
	require.Greater(t, lengths[2], lengths[1], "History should grow after second timer")
}

func Test_InstanceExecutionDetails_WithSubworkflow(t *testing.T) {
	r := registry.New()
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	hp := &testHistoryProvider{}
	e, err := newExecutor(r, i, hp)
	require.NoError(t, err)
	defer e.Close()

	var lengthBeforeSub, lengthAfterSub int64

	subworkflow := func(ctx wf.Context) error {
		return nil
	}

	workflow := func(ctx wf.Context) error {
		info := wf.InstanceExecutionDetails(ctx)
		lengthBeforeSub = info.HistoryLength

		wf.CreateSubWorkflowInstance[any](ctx, wf.SubWorkflowOptions{
			InstanceID: "subworkflow",
		}, subworkflow).Get(ctx)

		info = wf.InstanceExecutionDetails(ctx)
		lengthAfterSub = info.HistoryLength

		return nil
	}

	r.RegisterWorkflow(workflow)
	r.RegisterWorkflow(subworkflow)

	task := startWorkflowTask(i.InstanceID, workflow)

	result, err := e.ExecuteTask(context.Background(), task)
	require.NoError(t, err)

	// Should have scheduled the subworkflow
	require.Equal(t, int64(2), lengthBeforeSub)
	require.Greater(t, e.lastSequenceID, lengthBeforeSub)

	// Complete the subworkflow
	hp.history = append(hp.history, result.Executed...)
	swr, _ := converter.DefaultConverter.To(nil)
	task2 := continueTask(i.InstanceID, []*history.Event{
		history.NewPendingEvent(time.Now(), history.EventType_SubWorkflowCompleted, &history.SubWorkflowCompletedAttributes{
			Result: swr,
		}, history.ScheduleEventID(1)),
	}, result.Executed[len(result.Executed)-1].SequenceID)

	_, err = e.ExecuteTask(context.Background(), task2)
	require.NoError(t, err)

	require.Greater(t, lengthAfterSub, lengthBeforeSub)
}
