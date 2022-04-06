package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func NewRedisBackend(address, username, password string, db int, opts ...backend.BackendOption) (backend.Backend, error) {
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

	workflowQueue, err := taskqueue.New(client, "workflows")
	if err != nil {
		return nil, errors.Wrap(err, "could not create workflow task queue")
	}

	activityQueue, err := taskqueue.New(client, "activities")
	if err != nil {
		return nil, errors.Wrap(err, "could not create activity task queue")
	}

	rb := &redisBackend{
		rdb:     client,
		options: backend.ApplyOptions(opts...),

		workflowQueue: workflowQueue,
		activityQueue: activityQueue,
	}

	return rb, nil
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options backend.Options

	workflowQueue taskqueue.TaskQueue
	activityQueue taskqueue.TaskQueue
}

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	// TODO: Store signal event

	panic("unimplemented")
}
