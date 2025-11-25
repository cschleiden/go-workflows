package valkey

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/valkey-io/valkey-glide/go/v2/options"
)

func (vb *valkeyBackend) PrepareActivityQueues(ctx context.Context, queues []workflow.Queue) error {
	return vb.activityQueue.Prepare(ctx, vb.client, queues)
}

func (vb *valkeyBackend) GetActivityTask(ctx context.Context, queues []workflow.Queue) (*backend.ActivityTask, error) {
	activityTask, err := vb.activityQueue.Dequeue(ctx, vb.client, queues, vb.options.ActivityLockTimeout, vb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if activityTask == nil {
		return nil, nil
	}

	return &backend.ActivityTask{
		WorkflowInstance: activityTask.Data.Instance,
		Queue:            workflow.Queue(activityTask.Data.Queue),
		ID:               activityTask.TaskID,
		ActivityID:       activityTask.Data.ID,
		Event:            activityTask.Data.Event,
	}, nil
}

func (vb *valkeyBackend) ExtendActivityTask(ctx context.Context, task *backend.ActivityTask) error {
	if err := vb.activityQueue.Extend(ctx, vb.client, task.Queue, task.ID); err != nil {
		return err
	}

	return nil
}

func (vb *valkeyBackend) CompleteActivityTask(ctx context.Context, task *backend.ActivityTask, result *history.Event) error {
	instanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKey(task.WorkflowInstance))
	if err != nil {
		return err
	}

	// Marshal event data
	eventData, payload, err := marshalEvent(result)
	if err != nil {
		return err
	}

	activityQueueKeys := vb.activityQueue.Keys(task.Queue)
	workflowQueueKeys := vb.workflowQueue.Keys(workflow.Queue(instanceState.Queue))

	_, err = vb.client.InvokeScriptWithOptions(ctx, completeActivityTaskScript, options.ScriptOptions{
		Keys: []string{
			activityQueueKeys.SetKey,
			activityQueueKeys.StreamKey,
			vb.keys.pendingEventsKey(task.WorkflowInstance),
			vb.keys.payloadKey(task.WorkflowInstance),
			vb.workflowQueue.queueSetKey,
			workflowQueueKeys.SetKey,
			workflowQueueKeys.StreamKey,
		},
		Args: []string{
			task.ID,
			vb.activityQueue.groupName,
			result.ID,
			eventData,
			payload,
			vb.workflowQueue.groupName,
			instanceSegment(task.WorkflowInstance),
		},
	})

	if err != nil {
		return fmt.Errorf("completing activity task: %w", err)
	}

	return nil
}
