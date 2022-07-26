package redis

import (
	"context"
	"fmt"

	"github.com/ticctech/go-workflows/internal/core"
	"github.com/ticctech/go-workflows/internal/history"
	"github.com/ticctech/go-workflows/internal/task"
)

func (rb *redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	activityTask, err := rb.activityQueue.Dequeue(ctx, rb.rdb, rb.options.ActivityLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if activityTask == nil {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, rb.rdb, activityTask.Data.Instance.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("reading workflow instance for activity task: %w", err)
	}

	return &task.Activity{
		WorkflowInstance: activityTask.Data.Instance,
		Metadata:         instanceState.Metadata,
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

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, instance *core.WorkflowInstance, activityID string, event history.Event) error {
	p := rb.rdb.TxPipeline()

	if err := rb.addWorkflowInstanceEventP(ctx, p, instance, &event); err != nil {
		return err
	}

	// Unlock activity
	if _, err := rb.activityQueue.Complete(ctx, p, activityID); err != nil {
		return err
	}

	_, err := p.Exec(ctx)
	return err
}
