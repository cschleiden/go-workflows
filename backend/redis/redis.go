package redis

import (
	"context"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func NewRedisBackend(address, username, password string, db int, opts ...backend.BackendOption) backend.Backend {
	return &redisBackend{
		rdb: redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    []string{address},
			Username: username,
			Password: password,
			DB:       db,
		}),
		options: backend.ApplyOptions(opts...),
	}
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options backend.Options
}

func (rb *redisBackend) CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error {
	// Store instance with its state
	if err := storeInstance(ctx, rb.rdb, event.WorkflowInstance, &instanceState{
		InstanceID:  event.WorkflowInstance.GetInstanceID(),
		ExecutionID: event.WorkflowInstance.GetExecutionID(),
		State:       backend.WorkflowStateActive,
		CreatedAt:   time.Now(),
		CompletedAt: nil,
	}); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	// // Store pending event
	// pipe.RPush(ctx, pendingEventsKey(event.WorkflowInstance), "value", 0)

	// // Add instance to pending instances set
	// pipe.SAdd(ctx, pendingInstancesKey(), event.WorkflowInstance.GetInstanceID(), 0)

	return nil
}

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	var instanceID string

	getTask := func(tx *redis.Tx) error {
		// Find pending workflow instance from sorted set
		now := int(time.Now().Unix())
		cmd := tx.ZRangeByScoreWithScores(ctx, pendingInstancesKey(), &redis.ZRangeBy{
			// Get at most one task
			Count: 1,
			// Unlocked tasks have a score of 0 so start at -inf
			Min: "-inf",
			// Abandoned tasks will have an unlock-timestap in the past, so include those as well
			Max: strconv.Itoa(now),
		})
		if err := cmd.Err(); err != nil {
			return errors.Wrap(err, "could not get pending instance")
		}

		r, err := cmd.Result()
		if err != nil {
			return errors.Wrap(err, "could not get pending instance")
		}

		if len(r) == 0 {
			return nil
		}

		instanceID := r[0].Member.(string)

		// Mark instance as locked
		lockedUntil := time.Now().Add(rb.options.WorkflowLockTimeout)

		_, err = tx.Pipelined(ctx, func(p redis.Pipeliner) error {
			// Overwrite the key with the new score
			p.ZAdd(ctx, lockedInstancesKey(), &redis.Z{float64(lockedUntil.Unix()), instanceID})
			return nil
		})

		return err
	}

	for i := 0; i < 10; i++ {
		err := rb.rdb.Watch(ctx, getTask)
		if err == nil {
			// Success.
			break
		}

		if err == redis.TxFailedErr {
			// Optimistic lock lost. Retry.
			continue
		}

		// Return any other error.
		return nil, errors.Wrap(err, "could not find workflow task")
	}

	if instanceID == "" {
		return nil, nil
	}

	// TODO: Get pending events for instance
	// TODO: Get history for instance

	return &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance(instanceID, ""), // TODO: Read execution id,
		History:          []history.Event{},                        // TODO: Read history
		NewEvents:        []history.Event{},                        // TODO: Read new events
	}, nil
}

func (rb *redisBackend) ExtendWorkflowTask(ctx context.Context, instance core.WorkflowInstance) error {
	// TODO: Extend lock for instance

	panic("unimplemented")
}

func (rb *redisBackend) CompleteWorkflowTask(ctx context.Context, instance core.WorkflowInstance, state backend.WorkflowState, executedEvents []history.Event, activityEvents []history.Event, workflowEvents []history.WorkflowEvent) error {

	// TODO: Update instance state
	// TODO: Unlock instance - delete from ZSET
	// TODO: Add pending events to set
	// TODO: Add pending activity events to set
	// TODO: Add executed history events to history
	// TODO: Add instance back to pending instance

	panic("unimplemented")
}

func (rb *redisBackend) CancelWorkflowInstance(ctx context.Context, instance core.WorkflowInstance) error {
	panic("unimplemented")
}

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	// TODO: Store signal event

	panic("unimplemented")
}

func (rb *redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance core.WorkflowInstance) ([]history.Event, error) {
	panic("unimplemented")
}

func (rb *redisBackend) GetWorkflowInstanceState(ctx context.Context, instance core.WorkflowInstance) (backend.WorkflowState, error) {
	panic("unimplemented")
}

func (rb *redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	panic("unimplemented")
}

func (rb *redisBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	panic("unimplemented")
}

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, activityID string, event history.Event) error {
	panic("unimplemented")
}
