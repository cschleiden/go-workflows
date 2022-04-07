package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
)

func (rb *redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	activityTask, err := rb.activityQueue.Dequeue(ctx, rb.options.ActivityLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if activityTask == nil {
		return nil, nil
	}

	return &task.Activity{
		WorkflowInstance: core.NewWorkflowInstance(activityTask.Data.InstanceID, activityTask.Data.ExecutionID),
		ID:               activityTask.TaskID, // Use the queue generated ID here
		Event:            activityTask.Data.Event,
	}, nil
}

func (rb *redisBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	return rb.activityQueue.Extend(ctx, activityID)
}

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, instance *core.WorkflowInstance, activityID string, event history.Event) error {
	if err := rb.addWorkflowInstanceEvent(ctx, instance, &event); err != nil {
		return err
	}

	// Unlock activity
	return rb.activityQueue.Complete(ctx, activityID)
}
