package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/log"
	"github.com/go-redis/redis/v8"
)

type RedisOptions struct {
	backend.Options

	BlockTimeout time.Duration
}

type RedisBackendOption func(*RedisOptions)

func WithBlockTimeout(timeout time.Duration) RedisBackendOption {
	return func(o *RedisOptions) {
		o.BlockTimeout = timeout
	}
}

func WithBackendOptions(opts ...backend.BackendOption) RedisBackendOption {
	return func(o *RedisOptions) {
		for _, opt := range opts {
			opt(&o.Options)
		}
	}
}

func NewRedisBackend(address, username, password string, db int, opts ...RedisBackendOption) (*redisBackend, error) {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{address},
		Username:     username,
		Password:     password,
		DB:           db,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 30,
	})

	workflowQueue, err := taskqueue.New[any](client, "workflows")
	if err != nil {
		return nil, fmt.Errorf("creating workflow task queue: %w", err)
	}

	activityQueue, err := taskqueue.New[activityData](client, "activities")
	if err != nil {
		return nil, fmt.Errorf("creating activity task queue: %w", err)
	}

	// Default options
	options := &RedisOptions{
		Options:      backend.ApplyOptions(),
		BlockTimeout: time.Second * 5,
	}

	for _, opt := range opts {
		opt(options)
	}

	rb := &redisBackend{
		rdb:     client,
		options: options,

		workflowQueue: workflowQueue,
		activityQueue: activityQueue,
	}

	// Preload scripts here. Usually redis-go attempts to execute them first, and the if redis doesn't know
	// them, loads them. This doesn't work when using (transactional) pipelines, so eagerly load them on startup.
	ctx := context.Background()
	cmds := map[string]*redis.StringCmd{
		"addEventsToStreamCmd":   addEventsToStreamCmd.Load(ctx, rb.rdb),
		"addFutureEventCmd":      addFutureEventCmd.Load(ctx, rb.rdb),
		"futureEventsCmd":        futureEventsCmd.Load(ctx, rb.rdb),
		"removeFutureEventCmd":   removeFutureEventCmd.Load(ctx, rb.rdb),
		"removePendingEventsCmd": removePendingEventsCmd.Load(ctx, rb.rdb),
		"requeueInstanceCmd":     requeueInstanceCmd.Load(ctx, rb.rdb),
	}
	for name, cmd := range cmds {
		if cmd.Err() != nil {
			return nil, fmt.Errorf("loading redis script: %v %w", name, cmd.Err())
		}
	}

	return rb, nil
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options *RedisOptions

	workflowQueue taskqueue.TaskQueue[any]
	activityQueue taskqueue.TaskQueue[activityData]
}

type activityData struct {
	Instance *core.WorkflowInstance `json:"instance,omitempty"`
	ID       string                 `json:"id,omitempty"`
	Event    history.Event          `json:"event,omitempty"`
}

func (rb *redisBackend) Logger() log.Logger {
	return rb.options.Logger
}
