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

func addFutureEvent(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance, event *history.Event) error {
	futureEvent := &futureEvent{
		Instance: instance,
		Event:    event,
	}

	eventData, err := json.Marshal(futureEvent)
	if err != nil {
		return err
	}

	if err := rdb.ZAdd(ctx, futureEventsKey(), &redis.Z{
		Member: eventData,
		Score:  float64(event.VisibleAt.Unix()),
	}).Err(); err != nil {
		return fmt.Errorf("adding future event: %w", err)
	}

	return nil
}
