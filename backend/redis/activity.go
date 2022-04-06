package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func (rb *redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	var activityID string

	getAndLockTask := func(tx *redis.Tx) error {
		// Find pending workflow instance from sorted set
		now := int(time.Now().Unix())
		cmd := tx.ZRangeByScoreWithScores(ctx, activitiesKey(), &redis.ZRangeBy{
			// Get at most one task
			Count: 1,
			// Unlocked tasks have a score of 0 so start at -inf
			Min: "-inf",
			// Abandoned tasks will have an unlock-timestap in the past, so include those as well
			Max: strconv.Itoa(now),
		})
		if err := cmd.Err(); err != nil {
			return errors.Wrap(err, "could not get an activity task")
		}

		r, err := cmd.Result()
		if err != nil {
			return errors.Wrap(err, "could not get an activity task")
		}

		if len(r) == 0 {
			return nil
		}

		id := r[0].Member.(string)

		// Mark instance as locked
		lockedUntil := time.Now().Add(rb.options.ActivityLockTimeout)

		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			// Overwrite the key with the new score
			p.ZAdd(ctx, activitiesKey(), &redis.Z{Score: float64(lockedUntil.Unix()), Member: id})
			return nil
		})
		if err != nil {
			return err
		}

		activityID = id

		return nil
	}

	for i := 0; i < 10; i++ {
		err := rb.rdb.Watch(ctx, getAndLockTask, activitiesKey())
		if err == nil {
			// Success.
			break
		}

		if err == redis.TxFailedErr {
			// Optimistic lock lost. Retry.
			log.Println("TxFailed on activity task. Retrying...")
			continue
		}

		// Return any other error.
		return nil, errors.Wrap(err, "could not find activity task")
	}

	if activityID == "" {
		return nil, nil
	}

	activity, err := getActivity(ctx, rb.rdb, activityID)
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
	zcmd := rb.rdb.ZAddNX(ctx, pendingInstancesKey(), &redis.Z{Score: float64(0), Member: instance.GetInstanceID()})
	if added, err := zcmd.Result(); err != nil {
		return errors.Wrap(err, "could not add instance to locked instances set")
	} else if added == 0 {
		log.Println("Workflow instance already pending")
	}

	// Unlock activity
	rcmd := rb.rdb.ZRem(ctx, activitiesKey(), activityID)
	if removed, err := rcmd.Result(); err != nil {
		return errors.Wrap(err, "could not remove activity from locked activities set")
	} else if removed == 0 {
		return errors.Wrap(err, "activity already unlocked")
	}

	fmt.Println("Unlocked activity", activityID)

	// TODO: Remove state

	return nil
}

type ActivityData struct {
	InstanceID string        `json:"instance_id,omitempty"`
	ID         string        `json:"id,omitempty"`
	Event      history.Event `json:"event,omitempty"`
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

func queueActivityTask(ctx context.Context, rdb redis.UniversalClient, data *ActivityData) error {
	rdb.LPush(ctx, activitiesKey(), data.ID)

	return nil
}

func getActivityTask(ctx context.Context, rdb redis.UniversalClient) (*ActivityData, error) {
	cmd := rdb.BLMove(ctx, activitiesKey(), activitiesProcessingKey(), "RIGHT", "LEFT", time.Second*5)
	result, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			// Key does not exist or timeout, not an error
			return nil, nil
		}
	}

	data, err := getActivity(ctx, rdb, result)
	if err != nil {
		return nil, err
	}

	return data, nil
}
