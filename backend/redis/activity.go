package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type ActivityData struct {
	InstanceID string        `json:"instance_id,omitempty"`
	ID         string        `json:"id,omitempty"`
	Event      history.Event `json:"event,omitempty"`
}

func (rb *redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	// TODO: Make timeout configurable?
	activityID, err := rb.activityQueue.Dequeue(ctx, rb.options.ActivityLockTimeout, time.Second*5)
	if err != nil {
		return nil, err
	}

	if activityID == nil {
		return nil, nil
	}

	// Fetch activity data
	activity, err := getActivity(ctx, rb.rdb, *activityID)
	if err != nil {
		return nil, err
	}

	log.Println("Returning activity task", activity.ID)

	return &task.Activity{
		// TODO: Include execution id
		WorkflowInstance: core.NewWorkflowInstance(activity.InstanceID, ""),
		ID:               activity.ID,
		Event:            activity.Event,
	}, nil
}

func (rb *redisBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	panic("unimplemented")
}

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, activityID string, event history.Event) error {
	log.Println("Completing", activityID, event.ID, instance.GetInstanceID())

	// Deliver event to workflow instance
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	cmd := rb.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: pendingEventsKey(instance.GetInstanceID()),
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	})
	_, err = cmd.Result()
	if err != nil {
		return errors.Wrap(err, "could not add event to stream")
	}

	log.Println("Added event to stream", instance.GetInstanceID(), " event id ", event.ID)

	// Mark workflow instance as ready, if not already in queue
	if err := rb.workflowQueue.Enqueue(ctx, instance.GetInstanceID()); err != nil {
		return errors.Wrap(err, "could not queue workflow")
	}

	// Unlock activity
	if err := rb.activityQueue.Complete(ctx, activityID); err != nil {
		return err
	}

	fmt.Println("Unlocked activity", activityID)

	if err := removeActivity(ctx, rb.rdb, activityID); err != nil {
		return errors.Wrap(err, "could not remove activity")
	}

	return nil
}

func storeActivity(ctx context.Context, rdb redis.UniversalClient, data *ActivityData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "could not marshal activity data")
	}

	cmd := rdb.Set(ctx, activityKey(data.ID), string(b), 0)
	if err := cmd.Err(); err != nil {
		return errors.Wrap(err, "could not store activity")
	}

	return nil
}

func getActivity(ctx context.Context, rdb redis.UniversalClient, activityID string) (*ActivityData, error) {
	cmd := rdb.Get(ctx, activityKey(activityID))
	res, err := cmd.Result()
	if err != nil {
		return nil, err
	}

	var state ActivityData
	if err := json.Unmarshal([]byte(res), &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func removeActivity(ctx context.Context, rdb redis.UniversalClient, activityID string) error {
	if err := rdb.Del(ctx, activityKey(activityID)).Err(); err != nil {
		return errors.Wrap(err, "could not remove activity")
	}

	return nil
}
