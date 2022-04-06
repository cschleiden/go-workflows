package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func NewRedisBackend(address, username, password string, db int, opts ...backend.BackendOption) backend.Backend {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{address},
		Username: username,
		Password: password,
		DB:       db,
	})

	// TODO: Only for dev
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		panic(err)
	}

	return &redisBackend{
		rdb:     client,
		options: backend.ApplyOptions(opts...),
	}
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options backend.Options
}

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	// TODO: Store signal event

	panic("unimplemented")
}
