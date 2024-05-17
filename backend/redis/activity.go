package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
)

func (rb *redisBackend) GetActivityTask(ctx context.Context, namespaces []string) (*backend.ActivityTask, error) {
	activityTask, err := rb.activityQueue.Dequeue(ctx, rb.rdb, rb.options.ActivityLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if activityTask == nil {
		return nil, nil
	}

	return &backend.ActivityTask{
		WorkflowInstance: activityTask.Data.Instance,
		ID:               activityTask.TaskID, // Use the queue generated ID here
		Event:            activityTask.Data.Event,
	}, nil
}

func (rb *redisBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	p := rb.rdb.Pipeline()

	if err := rb.activityQueue.Extend(ctx, p, activityID); err != nil {
		return err
	}

	_, err := p.Exec(ctx)
	return err
}

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, instance *core.WorkflowInstance, activityID string, event *history.Event) error {
	p := rb.rdb.TxPipeline()

	if err := rb.addWorkflowInstanceEventP(ctx, p, instance, event); err != nil {
		return err
	}

	// Unlock activity
	if _, err := rb.activityQueue.Complete(ctx, p, activityID); err != nil {
		return err
	}

	_, err := p.Exec(ctx)
	return err
}
