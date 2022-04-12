package redis

import (
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/log"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
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

func NewRedisBackend(address, username, password string, db int, opts ...RedisBackendOption) (backend.Backend, error) {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{address},
		Username: username,
		Password: password,
		DB:       db,
	})

	workflowQueue, err := taskqueue.New[workflowTaskData](client, "workflows")
	if err != nil {
		return nil, errors.Wrap(err, "could not create workflow task queue")
	}

	activityQueue, err := taskqueue.New[activityData](client, "activities")
	if err != nil {
		return nil, errors.Wrap(err, "could not create activity task queue")
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

	return rb, nil
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options *RedisOptions

	workflowQueue taskqueue.TaskQueue[workflowTaskData]
	activityQueue taskqueue.TaskQueue[activityData]
}

type activityData struct {
	Instance *core.WorkflowInstance `json:"instance,omitempty"`
	ID       string                 `json:"id,omitempty"`
	Event    history.Event          `json:"event,omitempty"`
}

type workflowTaskData struct {
	LastPendingEventMessageID string `json:"last_pending_event_message_id,omitempty"`
}

func (rb *redisBackend) Logger() log.Logger {
	return rb.options.Logger
}
