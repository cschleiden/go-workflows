package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/workflow"
)

func (rb *redisBackend) PrepareActivityQueues(ctx context.Context, queues []workflow.Queue) error {
	return rb.activityQueue.Prepare(ctx, rb.rdb, queues)
}

func (rb *redisBackend) GetActivityTask(ctx context.Context, queues []workflow.Queue) (*backend.ActivityTask, error) {
	activityTask, err := rb.activityQueue.Dequeue(ctx, rb.rdb, queues, rb.options.ActivityLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if activityTask == nil {
		return nil, nil
	}

	return &backend.ActivityTask{
		WorkflowInstance: activityTask.Data.Instance,
		Queue:            workflow.Queue(activityTask.Data.Queue),
		ID:               activityTask.TaskID, // Use the queue generated ID here
		ActivityID:       activityTask.Data.ID,
		Event:            activityTask.Data.Event,
	}, nil
}

func (rb *redisBackend) ExtendActivityTask(ctx context.Context, task *backend.ActivityTask) error {
	p := rb.rdb.Pipeline()

	if err := rb.activityQueue.Extend(ctx, p, task.Queue, task.ID); err != nil {
		return err
	}

	_, err := p.Exec(ctx)
	return err
}

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, task *backend.ActivityTask, result *history.Event) error {
	instanceState, err := readInstance(ctx, rb.rdb, rb.keys.instanceKey(task.WorkflowInstance))
	if err != nil {
		return err
	}

	// Marshal event data
	eventData, payload, err := marshalEvent(result)
	if err != nil {
		return err
	}

	activityQueueKeys := rb.activityQueue.Keys(task.Queue)
	workflowQueueKeys := rb.workflowQueue.Keys(workflow.Queue(instanceState.Queue))

	err = completeActivityTaskCmd.Run(ctx, rb.rdb,
		[]string{
			activityQueueKeys.SetKey,
			activityQueueKeys.StreamKey,
			rb.keys.pendingEventsKey(task.WorkflowInstance),
			rb.keys.payloadKey(task.WorkflowInstance),
			rb.workflowQueue.queueSetKey,
			workflowQueueKeys.SetKey,
			workflowQueueKeys.StreamKey,
		},
		task.ID,
		rb.activityQueue.groupName,
		result.ID,
		eventData,
		payload,
		rb.workflowQueue.groupName,
		instanceSegment(task.WorkflowInstance),
	).Err()
	if err != nil {
		return fmt.Errorf("completing activity task: %w", err)
	}

	return nil
}
