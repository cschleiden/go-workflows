package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/metrics"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

var _ backend.Backend = (*redisBackend)(nil)

func NewRedisBackend(client redis.UniversalClient, opts ...RedisBackendOption) (*redisBackend, error) {
	workflowQueue, err := newTaskQueue[any](client, "workflows")
	if err != nil {
		return nil, fmt.Errorf("creating workflow task queue: %w", err)
	}

	activityQueue, err := newTaskQueue[activityData](client, "activities")
	if err != nil {
		return nil, fmt.Errorf("creating activity task queue: %w", err)
	}

	// Default options
	options := &RedisOptions{
		Options:      backend.ApplyOptions(),
		BlockTimeout: time.Second * 2,
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

	// Preload scripts here. Usually redis-go attempts to execute them first, and if redis doesn't know
	// them, loads them. This doesn't work when using (transactional) pipelines, so eagerly load them on startup.
	ctx := context.Background()
	cmds := map[string]*redis.StringCmd{
		"addEventsToStreamCmd":   addEventsToStreamCmd.Load(ctx, rb.rdb),
		"addFutureEventCmd":      addFutureEventCmd.Load(ctx, rb.rdb),
		"futureEventsCmd":        futureEventsCmd.Load(ctx, rb.rdb),
		"removeFutureEventCmd":   removeFutureEventCmd.Load(ctx, rb.rdb),
		"removePendingEventsCmd": removePendingEventsCmd.Load(ctx, rb.rdb),
		"requeueInstanceCmd":     requeueInstanceCmd.Load(ctx, rb.rdb),
		"deleteInstanceCmd":      deleteCmd.Load(ctx, rb.rdb),
		"expireInstanceCmd":      expireCmd.Load(ctx, rb.rdb),
	}
	for name, cmd := range cmds {
		// fmt.Println(name, cmd.Val())

		if cmd.Err() != nil {
			return nil, fmt.Errorf("loading redis script: %v %w", name, cmd.Err())
		}
	}

	return rb, nil
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options *RedisOptions

	workflowQueue *taskQueue[any]
	activityQueue *taskQueue[activityData]
}

type activityData struct {
	Instance *core.WorkflowInstance `json:"instance,omitempty"`
	ID       string                 `json:"id,omitempty"`
	Event    *history.Event         `json:"event,omitempty"`
}

func (rb *redisBackend) Logger() *slog.Logger {
	return rb.options.Logger
}

func (rb *redisBackend) Metrics() metrics.Client {
	return rb.options.Metrics.WithTags(metrics.Tags{metrickeys.Backend: "redis"})
}

func (rb *redisBackend) Tracer() trace.Tracer {
	return rb.options.TracerProvider.Tracer(backend.TracerName)
}

func (rb *redisBackend) Converter() converter.Converter {
	return rb.options.Converter
}

func (rb *redisBackend) ContextPropagators() []contextpropagation.ContextPropagator {
	return rb.options.ContextPropagators
}

func (rb *redisBackend) Close() error {
	return rb.rdb.Close()
}
