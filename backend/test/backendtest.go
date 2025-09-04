package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func BackendTest(t *testing.T, setup func(options ...backend.BackendOption) TestBackend, teardown func(b TestBackend)) {
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
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: workflow.QueueDefault,
					}),
				)
				require.NoError(t, err)
			},
		},
		{
			name: "CreateWorkflowInstance_SameInstanceIDErrors",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()
				executionID1 := uuid.NewString()
				executionID2 := uuid.NewString()

				err := b.CreateWorkflowInstance(ctx,
					core.NewWorkflowInstance(instanceID, executionID1),
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: workflow.QueueDefault,
					}),
				)
				require.NoError(t, err)

				err = b.CreateWorkflowInstance(
					ctx,
					core.NewWorkflowInstance(instanceID, executionID2),
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: workflow.QueueDefault,
					}),
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
					history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue:    workflow.QueueDefault,
						Metadata: metadata,
					}),
				)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))
				task, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)
				require.NotNil(t, task)

				require.NotNil(t, task.Metadata)
				require.Equal(t, *metadata, *task.Metadata)
			},
		},
		{
			name: "RemoveWorkflowInstance_ErrorWhenInstanceInProgress",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()
				wfi := core.NewWorkflowInstance(instanceID, uuid.NewString())

				err := b.CreateWorkflowInstance(
					ctx, wfi, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: workflow.QueueDefault,
					}),
				)
				require.NoError(t, err)

				err = b.RemoveWorkflowInstance(ctx, wfi)
				require.Error(t, err)
				require.Equal(t, backend.ErrInstanceNotFinished, err)
			},
		},
		{
			name: "RemoveWorkflowInstance_ErrorWhenInstanceDoesNotExist",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instanceID := uuid.NewString()
				wfi := core.NewWorkflowInstance(instanceID, uuid.NewString())

				err := b.RemoveWorkflowInstance(ctx, wfi)
				require.Error(t, err)
				require.Equal(t, backend.ErrInstanceNotFound, err)
			},
		},
		{
			name: "GetWorkflowTask_ReturnsNilWhenTimeout",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
				defer cancel()

				time.Sleep(1 * time.Millisecond)
				task, _ := b.GetWorkflowTask(ctx, queues)
				require.Nil(t, task)
			},
		},
		{
			name: "GetWorkflowTask_ReturnsTask",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(
					ctx, wfi, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: workflow.QueueDefault,
					}),
				)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))
				task, err := b.GetWorkflowTask(ctx, queues)

				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, wfi.InstanceID, task.WorkflowInstance.InstanceID)
			},
		},
		{
			name: "GetWorkflowTask_ReturnsTaskFromGivenQueue",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(
					ctx, wfi, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: "customQueue",
					}),
				)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				tctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
				task, err := b.GetWorkflowTask(tctx, queues)
				cancel()
				require.True(t, err == nil || errors.Is(err, context.DeadlineExceeded))
				require.Nil(t, task)

				customQueues := []workflow.Queue{"customQueue"}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, customQueues))

				task, err = b.GetWorkflowTask(ctx, customQueues)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, wfi.InstanceID, task.WorkflowInstance.InstanceID)
				require.Equal(t, workflow.Queue("customQueue"), task.Queue)
			},
		},
		{
			name: "GetWorkflowTask_LocksTask",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(
					ctx, wfi, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
						Queue: workflow.QueueDefault,
					}),
				)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				// Get and lock only task
				task, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)
				require.NotNil(t, task)

				// First task is locked, second call should return nil
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
				defer cancel()

				task, err = b.GetWorkflowTask(ctx, []workflow.Queue{workflow.QueueDefault, core.QueueSystem})
				require.Nil(t, task)
				require.True(t, err == nil || errors.Is(err, context.DeadlineExceeded))
			},
		},
		{
			name: "CompleteWorkflowTask_ReturnsErrorIfNotLocked",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, wfi, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
					Queue: workflow.QueueDefault,
				}))
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				tk, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)
				require.NotNil(t, tk)

				// Complete workflow task
				err = b.CompleteWorkflowTask(ctx, tk, core.WorkflowInstanceStateActive, tk.NewEvents, []*history.Event{}, []*history.Event{}, []*history.WorkflowEvent{})
				require.NoError(t, err)

				// Task is already completed, this should error
				err = b.CompleteWorkflowTask(ctx, tk, core.WorkflowInstanceStateActive, tk.NewEvents, []*history.Event{}, []*history.Event{}, []*history.WorkflowEvent{})
				require.Error(t, err)
			},
		},
		{
			name: "CompleteWorkflowTask_AddsNewEventsToHistory",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				startedEvent := history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
					Queue: workflow.QueueDefault,
				})
				activityScheduledEvent := history.NewPendingEvent(time.Now(), history.EventType_ActivityScheduled, &history.ActivityScheduledAttributes{}, history.ScheduleEventID(1))

				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, wfi, startedEvent)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				task, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)

				taskStartedEvent := history.NewPendingEvent(time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})
				events := []*history.Event{
					taskStartedEvent,
					startedEvent,
					activityScheduledEvent,
				}

				sequenceID := int64(1)
				for i := range events {
					sequenceID++
					events[i].SequenceID = sequenceID
				}

				activityEvents := []*history.Event{
					activityScheduledEvent,
				}

				workflowEvents := []*history.WorkflowEvent{}

				err = b.CompleteWorkflowTask(ctx, task, core.WorkflowInstanceStateActive, events, activityEvents, []*history.Event{}, workflowEvents)
				require.NoError(t, err)

				time.Sleep(time.Second)

				h, err := b.GetWorkflowInstanceHistory(ctx, wfi, nil)
				require.NoError(t, err)
				require.Len(t, h, len(events))
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
				startedEvent := history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
					Queue:    workflow.QueueDefault,
					Name:     "some-workflow",
					Inputs:   []payload.Payload{},
					Metadata: &metadata.WorkflowMetadata{},
				})

				wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
				err := b.CreateWorkflowInstance(ctx, wfi, startedEvent)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				task, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)
				require.NotNil(t, task)

				events := []*history.Event{
					history.NewHistoryEvent(-1, time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{}),
					startedEvent,
					history.NewHistoryEvent(-1, time.Now(), history.EventType_WorkflowExecutionFinished, &history.ExecutionCompletedAttributes{}),
				}

				sequenceID := int64(1)
				for i := range events {
					sequenceID++
					events[i].SequenceID = sequenceID
				}

				err = b.CompleteWorkflowTask(ctx, task, core.WorkflowInstanceStateFinished, events, []*history.Event{}, []*history.Event{}, []*history.WorkflowEvent{})
				require.NoError(t, err)

				time.Sleep(time.Second)

				db := b.(diag.Backend)
				s, err := db.GetWorkflowInstance(ctx, wfi)
				require.NoError(t, err)
				require.Equal(t, core.WorkflowInstanceStateFinished, s.State)
				require.NotNil(t, s.CompletedAt)
			},
		},
		{
			name: "CompleteWorkflowTask_SendsInstanceEvents",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())

				subInstance1 := core.NewSubWorkflowInstance(uuid.NewString(), uuid.NewString(), instance, 1)
				startWorkflow(t, ctx, b, subInstance1)

				// Create parent instance
				err := b.CreateWorkflowInstance(ctx, instance, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
					Queue: workflow.QueueDefault,
				}))
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

				// Simulate context and sub-workflow cancellation
				task, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)
				err = b.CompleteWorkflowTask(ctx, task, core.WorkflowInstanceStateActive, task.NewEvents, []*history.Event{}, []*history.Event{}, []*history.WorkflowEvent{
					{
						WorkflowInstance: subInstance1,
						HistoryEvent: history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionCanceled, &history.SubWorkflowCancellationRequestedAttributes{
							SubWorkflowInstance: subInstance1,
						}),
					},
				})
				require.NoError(t, err)

				task, err = b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, subInstance1, task.WorkflowInstance)
				require.Equal(t, history.EventType_WorkflowExecutionCanceled, task.NewEvents[len(task.NewEvents)-1].Type)
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
				startWorkflow(t, ctx, b, instance)

				err := c.CancelWorkflowInstance(ctx, instance)
				require.NoError(t, err)

				queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
				task, err := b.GetWorkflowTask(ctx, queues)
				require.NoError(t, err)

				require.Equal(t, history.EventType_WorkflowExecutionCanceled, task.NewEvents[len(task.NewEvents)-1].Type)
			},
		},
		{
			name: "GetActivityTask_ReturnsNilWhenTimeout",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
				defer cancel()

				task, _ := b.GetActivityTask(ctx, []workflow.Queue{workflow.QueueDefault})
				require.Nil(t, task)
			},
		},
		{
			name: "GetActivityTask_ReturnsTaskFromQueues",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				wfiDefault := runWorkflowWithActivity(t, ctx, b, workflow.QueueDefault, workflow.QueueDefault)

				require.NoError(t, b.PrepareActivityQueues(ctx, []workflow.Queue{workflow.QueueDefault, "custom"}))

				tctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
				task, err := b.GetActivityTask(tctx, []workflow.Queue{workflow.QueueDefault})
				cancel()
				require.True(t, err == nil || errors.Is(err, context.DeadlineExceeded))
				require.NotNil(t, task)
				require.Equal(t, wfiDefault.InstanceID, task.WorkflowInstance.InstanceID)

				customQueue := workflow.Queue("custom")
				wfiCustom := runWorkflowWithActivity(t, ctx, b, core.QueueDefault, customQueue)

				task, err = b.GetActivityTask(ctx, []workflow.Queue{customQueue})
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, wfiCustom.InstanceID, task.WorkflowInstance.InstanceID)
			},
		},
		{
			name: "CompleteActivityTask_DeliversResultBackToWorkflowQueue",
			f: func(t *testing.T, ctx context.Context, b backend.Backend) {
				activityQueue := workflow.Queue("custom")
				wfiDefault := runWorkflowWithActivity(t, ctx, b, workflow.QueueDefault, activityQueue)

				require.NoError(t, b.PrepareActivityQueues(ctx, []workflow.Queue{workflow.QueueDefault, activityQueue}))

				task, err := b.GetActivityTask(ctx, []workflow.Queue{activityQueue})
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, wfiDefault.InstanceID, task.WorkflowInstance.InstanceID)

				require.NoError(t,
					b.CompleteActivityTask(ctx, task, history.NewHistoryEvent(1, time.Now(), history.EventType_ActivityCompleted, &history.ActivityCompletedAttributes{})),
				)

				wfTask, err := b.GetWorkflowTask(ctx, []workflow.Queue{workflow.QueueDefault})
				require.NoError(t, err)
				require.NotNil(t, wfTask)
				require.Equal(t, history.EventType_ActivityCompleted, wfTask.NewEvents[len(wfTask.NewEvents)-1].Type)
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

func startWorkflow(t *testing.T, ctx context.Context, b backend.Backend, instance *core.WorkflowInstance) {
	err := b.CreateWorkflowInstance(
		ctx, instance, history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
			Queue: workflow.QueueDefault,
		}))
	require.NoError(t, err)

	queues := []workflow.Queue{workflow.QueueDefault, core.QueueSystem}
	require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

	// Get task to clear initial event
	task, err := b.GetWorkflowTask(ctx, queues)
	require.NoError(t, err)
	require.NotNil(t, task)

	err = b.CompleteWorkflowTask(
		ctx, task, core.WorkflowInstanceStateActive, task.NewEvents, []*history.Event{}, []*history.Event{}, []*history.WorkflowEvent{})
	require.NoError(t, err)
}

func runWorkflowWithActivity(t *testing.T, ctx context.Context, b backend.Backend, queue workflow.Queue, activityQueue workflow.Queue) *workflow.Instance {
	startedEvent := history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
		Queue: workflow.QueueDefault,
	})
	activityScheduledEvent := history.NewPendingEvent(time.Now(), history.EventType_ActivityScheduled, &history.ActivityScheduledAttributes{
		Queue: activityQueue,
	}, history.ScheduleEventID(1))

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	err := b.CreateWorkflowInstance(ctx, wfi, startedEvent)
	require.NoError(t, err)

	queues := []workflow.Queue{queue}
	require.NoError(t, b.PrepareWorkflowQueues(ctx, queues))

	task, err := b.GetWorkflowTask(ctx, queues)
	require.NoError(t, err)

	taskStartedEvent := history.NewPendingEvent(time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})
	events := []*history.Event{
		taskStartedEvent,
		startedEvent,
		activityScheduledEvent,
	}

	sequenceID := int64(1)
	for i := range events {
		sequenceID++
		events[i].SequenceID = sequenceID
	}

	activityEvents := []*history.Event{
		activityScheduledEvent,
	}

	workflowEvents := []*history.WorkflowEvent{}

	err = b.CompleteWorkflowTask(ctx, task, core.WorkflowInstanceStateActive, events, activityEvents, []*history.Event{}, workflowEvents)
	require.NoError(t, err)

	return wfi
}
