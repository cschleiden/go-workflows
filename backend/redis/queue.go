package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type queue struct {
	item string
	keys *keys
	rdb  redis.UniversalClient
}

func newQueue(rdb redis.UniversalClient, item string) *queue {
	return &queue{
		item,
		queueKeys(item),
		rdb,
	}
}

func (q *queue) Enqueue(ctx context.Context, id string) error {
	return q.rdb.LPush(ctx, q.keys.queue, id).Err()
}

func (q *queue) Dequeue(ctx context.Context, lease, timeout time.Duration) (*string, error) {
	cmd := q.rdb.BLMove(ctx, q.keys.queue, q.keys.processing, "RIGHT", "LEFT", timeout)
	id, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}

		return nil, errors.Wrap(err, "could not get activity task")
	}

	// lockTimeout := time.Now().Add(lease).Unix()
	// if q.rdb.ZAdd(ctx, activityKeys.lease, &redis.Z{
	// 	Score:  float64(lockTimeout),
	// 	Member: id,
	// }).Err() != nil {
	// 	return nil, errors.Wrap(err, "could not store lease for activity")
	// }

	return &id, nil
}
func (q *queue) Complete(ctx context.Context, id string) error {
	return q.rdb.LRem(ctx, q.keys.processing, 1, id).Err()
}
