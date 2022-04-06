package redis

import (
	"context"
	"encoding/json"

	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func addEventToStream(ctx context.Context, rdb redis.UniversalClient, streamKey string, event *history.Event) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	}).Err(); err != nil {
		return errors.Wrap(err, "could not add event to stream")
	}

	return nil
}
