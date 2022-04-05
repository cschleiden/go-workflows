package redis

import (
	"context"
	"encoding/json"

	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

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
