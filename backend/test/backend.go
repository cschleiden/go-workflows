package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	ta "github.com/cschleiden/go-workflows/internal/task"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func BackendTest(t *testing.T, setup func() backend.Backend, teardown func(b backend.Backend)) {
	tests := []struct {
		name string
		f    func(t *testing.T, ctx context.Context, b backend.Backend)
	}{
		{
			name: "Test_GetWorkflowTask_ReturnsNilWhenTimeout",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
				defer cancel()

				task, _ := b.GetWorkflowTask(ctx)
				require.Nil(t, task)
			},
		},
		{
			name: "Test_GetActivityTask_ReturnsNilWhenTimeout",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
				defer cancel()

				task, _ := b.GetActivityTask(ctx)
				require.Nil(t, task)
			},
		},
		{
			name: "Test_CreateWorkflowInstance_DoesNotError",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()

				err := b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: core.NewWorkflowInstance(instanceID, uuid.NewString()),
					HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				})
				require.NoError(t, err)
			},
		},
		{
			name: "Test_CreateWorkflowInstance_SameInstanceIDErrors",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()
				executionID := uuid.NewString()

				err := b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
					HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				})
				require.NoError(t, err)

				err = b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
					HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				})
				require.Error(t, err)
			},
		},
		{
			name: "Test_GetWorkflowTask_ReturnsTask",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: wfi,
					HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				})
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)

				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, wfi.InstanceID, task.WorkflowInstance.InstanceID)
			},
		},
		{
			name: "Test_GetWorkflowTask_LocksTask",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: wfi,
					HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				})
				require.Nil(t, err)

				// Get and lock only task
				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)
				require.NotNil(t, task)

				// First task is locked, second call should return nil
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
				defer cancel()

				task, err = b.GetWorkflowTask(ctx)

				require.NoError(t, err)
				require.Nil(t, task)
			},
		},
		{
			name: "Test_CompleteWorkflowTask_ReturnsErrorIfNotLocked",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: wfi,
					HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				})
				require.NoError(t, err)

				err = b.CompleteWorkflowTask(ctx, "taskID", wfi, backend.WorkflowStateActive, []history.Event{}, []history.Event{}, []history.WorkflowEvent{})

				require.Error(t, err)
			},
		},
		{
			name: "Test_CompleteWorkflowTask_AddsNewEventsToHistory",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				startedEvent := history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{})
				activityScheduledEvent := history.NewHistoryEvent(time.Now(), history.EventType_ActivityScheduled, &history.ActivityScheduledAttributes{}, history.ScheduleEventID(1))
				activityCompletedEvent := history.NewHistoryEvent(time.Now(), history.EventType_ActivityCompleted, &history.ActivityCompletedAttributes{}, history.ScheduleEventID(1))

				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, history.WorkflowEvent{
					WorkflowInstance: wfi,
					HistoryEvent:     startedEvent,
				})
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)

				taskStartedEvent := history.NewHistoryEvent(time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})
				taskFinishedEvent := history.NewHistoryEvent(time.Now(), history.EventType_WorkflowTaskFinished, &history.WorkflowTaskFinishedAttributes{})
				events := []history.Event{
					taskStartedEvent,
					startedEvent,
					activityScheduledEvent,
					taskFinishedEvent,
				}

				activityEvents := []history.Event{
					activityScheduledEvent,
				}

				workflowEvents := []history.WorkflowEvent{
					{
						WorkflowInstance: wfi,
						HistoryEvent:     activityCompletedEvent,
					},
				}

				err = b.CompleteWorkflowTask(ctx, task.ID, wfi, backend.WorkflowStateActive, events, activityEvents, workflowEvents)
				require.NoError(t, err)

				time.Sleep(time.Second)

				task, err = b.GetWorkflowTask(ctx)
				require.NotEqual(t, ta.Continuation, task.Kind, "Expect full task")
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, len(events), len(task.History))
				// Only compare event types
				for i, expected := range events {
					require.Equal(t, expected.Type, task.History[i].Type)
				}
				require.Len(t, task.NewEvents, 1)
				require.Equal(t, activityCompletedEvent.Type, task.NewEvents[0].Type, "Expected new events to be returned")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := setup()
			ctx := context.Background()
			tt.f(t, ctx, b)
			if teardown != nil {
				teardown(b)
			}
		})
	}
}
