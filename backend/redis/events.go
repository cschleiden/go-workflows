package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func addEventToStream(ctx context.Context, rdb redis.UniversalClient, streamKey string, event *history.Event) (*string, error) {
	eventData, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	msgID, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("adding event to stream: %w", err)
	}

	return &msgID, nil
}

// KEYS[1] - future event zset key
// KEYS[2] - future event key
// ARGV[1] - timestamp
// ARGV[2] - event payload
var addFutureEventCmd = redis.NewScript(`
	redis.call("ZADD", KEYS[1], ARGV[1], KEYS[2])
	redis.call("SET", KEYS[2], ARGV[2])
`)

func addFutureEvent(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance, event *history.Event) error {
	futureEvent := &futureEvent{
		Instance: instance,
		Event:    event,
	}

	eventData, err := json.Marshal(futureEvent)
	if err != nil {
		return err
	}

	if err := addFutureEventCmd.Run(
		ctx,
		rdb,
		[]string{futureEventsKey(), futureEventKey(instance.InstanceID, event.ScheduleEventID)},
		event.VisibleAt.Unix(),
		string(eventData),
	).Err(); err != nil && err != redis.Nil {
		return fmt.Errorf("adding future event: %w", err)
	}

	return nil
}

// KEYS[1] - future event zset key
// KEYS[2] - future event key
var removeFutureEventCmd = redis.NewScript(`
	redis.call("ZREM", KEYS[1], KEYS[2])
	redis.call("DEL", KEYS[2])
`)

func removeFutureEvent(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance, event *history.Event) error {
	key := futureEventKey(instance.InstanceID, event.ScheduleEventID)

	if err := removeFutureEventCmd.Run(ctx, rdb, []string{futureEventsKey(), key}).Err(); err != nil && err != redis.Nil {
		return fmt.Errorf("removing future event: %w", err)
	}

	return nil
}
