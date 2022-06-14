package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func BackendTest(t *testing.T, setup func() TestBackend, teardown func(b TestBackend)) {
	tests := []struct {
		name string
		f    func(t *testing.T, ctx context.Context, b backend.Backend)
	}{
		{
			name: "CreateWorkflowInstance_DoesNotError",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()

				err := b.CreateWorkflowInstance(
					ctx,
					core.NewWorkflowInstance(instanceID, uuid.NewString()),
					nil,
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				)
				require.NoError(t, err)
			},
		},
		{
			name: "CreateWorkflowInstance_SameInstanceIDErrors",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()
				executionID := uuid.NewString()

				err := b.CreateWorkflowInstance(ctx,
					core.NewWorkflowInstance(instanceID, executionID),
					nil,
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				)
				require.NoError(t, err)

				err = b.CreateWorkflowInstance(
					ctx,
					core.NewWorkflowInstance(instanceID, executionID),
					nil,
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				)
				require.Error(t, err)
				require.ErrorIs(t, err, backend.ErrInstanceAlreadyExists)
			},
		},
		{
			name: "CreateWorkflowInstance_Metadata",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()

				metadata := &workflow.Metadata{}

				wfi := core.NewWorkflowInstance(instanceID, uuid.NewString())

				err := b.CreateWorkflowInstance(
					ctx,
					wfi,
					metadata,
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				)
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)
				require.NotNil(t, task)

				require.NotNil(t, task.Metadata)
				require.Equal(t, *metadata, *task.Metadata)
			},
		},
		{
			name: "GetWorkflowTask_ReturnsNilWhenTimeout",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
				defer cancel()

				task, err := b.GetWorkflowTask(ctx)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.Nil(t, task)
			},
		},
		{
			name: "GetWorkflowTask_ReturnsTask",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(
					ctx, wfi, nil, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				)
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)

				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, wfi.InstanceID, task.WorkflowInstance.InstanceID)
			},
		},
		{
			name: "GetWorkflowTask_LocksTask",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(
					ctx, wfi, nil, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
				)
				require.Nil(t, err)

				// Get and lock only task
				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)
				require.NotNil(t, task)

				// First task is locked, second call should return nil
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
				defer cancel()

				task, err = b.GetWorkflowTask(ctx)
				require.Nil(t, task)
				require.True(t, err == nil || errors.Is(err, context.DeadlineExceeded))
			},
		},
		{
			name: "CompleteWorkflowTask_ReturnsErrorIfNotLocked",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, wfi, nil, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}))
				require.NoError(t, err)

				tk, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)
				require.NotNil(t, tk)

				// Complete workflow task
				err = b.CompleteWorkflowTask(ctx, tk, wfi, backend.WorkflowStateActive, tk.NewEvents, []history.Event{}, []history.Event{}, []history.WorkflowEvent{})
				require.NoError(t, err)

				// Task is already completed, this should error
				err = b.CompleteWorkflowTask(ctx, tk, wfi, backend.WorkflowStateActive, tk.NewEvents, []history.Event{}, []history.Event{}, []history.WorkflowEvent{})
				require.Error(t, err)
			},
		},
		{
			name: "CompleteWorkflowTask_AddsNewEventsToHistory",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				startedEvent := history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{})
				activityScheduledEvent := history.NewPendingEvent(time.Now(), history.EventType_ActivityScheduled, &history.ActivityScheduledAttributes{}, history.ScheduleEventID(1))

				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, wfi, nil, startedEvent)
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)

				taskStartedEvent := history.NewPendingEvent(time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})
				events := []history.Event{
					taskStartedEvent,
					startedEvent,
					activityScheduledEvent,
				}

				sequenceID := int64(1)
				for i := range events {
					sequenceID++
					events[i].SequenceID = sequenceID
				}

				activityEvents := []history.Event{
					activityScheduledEvent,
				}

				workflowEvents := []history.WorkflowEvent{}

				err = b.CompleteWorkflowTask(ctx, task, wfi, backend.WorkflowStateActive, events, activityEvents, []history.Event{}, workflowEvents)
				require.NoError(t, err)

				time.Sleep(time.Second)

				h, err := b.GetWorkflowInstanceHistory(ctx, wfi, nil)
				require.NoError(t, err)
				require.Equal(t, len(events), len(h))
				for i, event := range events {
					require.Equal(t, event.ID, h[i].ID)
					require.Equal(t, event.Type, h[i].Type)
					require.Equal(t, event.Attributes, h[i].Attributes)
				}
			},
		},
		{
			name: "CompleteWorkflowTask_SetsCompletedAtWhenFinished",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				startedEvent := history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{})

				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, wfi, nil, startedEvent)
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)

				events := []history.Event{
					history.NewHistoryEvent(-1, time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{}),
					startedEvent,
					history.NewHistoryEvent(-1, time.Now(), history.EventType_WorkflowExecutionFinished, &history.ExecutionCompletedAttributes{}),
				}

				sequenceID := int64(1)
				for i := range events {
					sequenceID++
					events[i].SequenceID = sequenceID
				}

				err = b.CompleteWorkflowTask(ctx, task, wfi, backend.WorkflowStateFinished, events, []history.Event{}, []history.Event{}, []history.WorkflowEvent{})
				require.NoError(t, err)

				time.Sleep(time.Second)

				db := b.(diag.Backend)
				s, err := db.GetWorkflowInstance(ctx, wfi.InstanceID)
				require.NoError(t, err)
				require.Equal(t, backend.WorkflowStateFinished, s.State)
				require.NotNil(t, s.CompletedAt)
			},
		},
		{
			name: "SignalWorkflow_ErrorWhenInstanceDoesNotExist",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				c := client.New(b)
				err := c.SignalWorkflow(ctx, "does-not-exist", "signal", "value")
				require.Error(t, err)
				require.Equal(t, backend.ErrInstanceNotFound, err)
			},
		},
		{
			name: "CancelWorkflow_ErrorWhenInstanceDoesNotExist",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				c := client.New(b)
				err := c.CancelWorkflowInstance(ctx, core.NewWorkflowInstance(uuid.NewString(), uuid.NewString()))
				require.Error(t, err)
				require.Equal(t, backend.ErrInstanceNotFound, err)
			},
		},
		{
			name: "CancelWorkflow_AddsCancelEventToPendingEvents",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				c := client.New(b)
				instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				startWorkflow(t, ctx, b, c, instance)

				err := c.CancelWorkflowInstance(ctx, instance)
				require.NoError(t, err)

				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)

				require.Equal(t, history.EventType_WorkflowExecutionCanceled, task.NewEvents[len(task.NewEvents)-1].Type)
			},
		},
		{
			name: "CompleteWorkflowTask_SendsInstanceEvents",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				c := client.New(b)
				instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())

				subInstance1 := core.NewSubWorkflowInstance(uuid.NewString(), uuid.NewString(), instance.InstanceID, 1)
				startWorkflow(t, ctx, b, c, subInstance1)

				// Create parent instance
				err := b.CreateWorkflowInstance(ctx, instance, nil, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}))
				require.NoError(t, err)

				// Simulate context and sub-workflow cancellation
				task, err := b.GetWorkflowTask(ctx)
				require.NoError(t, err)
				err = b.CompleteWorkflowTask(ctx, task, instance, backend.WorkflowStateActive, task.NewEvents, []history.Event{}, []history.Event{}, []history.WorkflowEvent{
					{
						WorkflowInstance: subInstance1,
						HistoryEvent: history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionCanceled, &history.SubWorkflowCancellationRequestedAttributes{
							SubWorkflowInstance: subInstance1,
						}),
					},
				})
				require.NoError(t, err)

				task, err = b.GetWorkflowTask(ctx)
				require.NoError(t, err)
				require.Equal(t, subInstance1, task.WorkflowInstance)
				require.Equal(t, history.EventType_WorkflowExecutionCanceled, task.NewEvents[len(task.NewEvents)-1].Type)
			},
		},
		{
			name: "GetActivityTask_ReturnsNilWhenTimeout",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
				defer cancel()

				task, _ := b.GetActivityTask(ctx)
				require.Nil(t, task)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := setup()
			ctx := context.Background()

			t.Cleanup(func() {
				if teardown != nil {
					teardown(b)
				}
			})

			tt.f(t, ctx, b)
		})
	}
}

func startWorkflow(t *testing.T, ctx context.Context, b backend.Backend, c client.Client, instance *core.WorkflowInstance) {
	err := b.CreateWorkflowInstance(ctx, instance, nil, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}))
	require.NoError(t, err)

	// Get task to clear initial event
	task, err := b.GetWorkflowTask(ctx)
	require.NoError(t, err)

	err = b.CompleteWorkflowTask(ctx, task, instance, backend.WorkflowStateActive, task.NewEvents, []history.Event{}, []history.Event{}, []history.WorkflowEvent{})
	require.NoError(t, err)
}
