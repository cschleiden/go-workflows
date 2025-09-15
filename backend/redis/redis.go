package redis

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

var _ backend.Backend = (*redisBackend)(nil)

//go:embed scripts
var luaScripts embed.FS

var (
	createWorkflowInstanceCmd *redis.Script
	completeWorkflowTaskCmd   *redis.Script
	futureEventsCmd           *redis.Script
	expireWorkflowInstanceCmd *redis.Script
)

func NewRedisBackend(client redis.UniversalClient, opts ...RedisBackendOption) (*redisBackend, error) {
	// Default options
	options := &RedisOptions{
		Options:      backend.ApplyOptions(),
		BlockTimeout: time.Second * 2,
	}

	for _, opt := range opts {
		opt(options)
	}

	ctx := context.Background()

	workflowQueue, err := newTaskQueue[workflowData](ctx, client, options.KeyPrefix, "workflows", options.WorkerName)
	if err != nil {
		return nil, fmt.Errorf("creating workflow task queue: %w", err)
	}

	activityQueue, err := newTaskQueue[activityData](ctx, client, options.KeyPrefix, "activities", options.WorkerName)
	if err != nil {
		return nil, fmt.Errorf("creating activity task queue: %w", err)
	}

	rb := &redisBackend{
		rdb:     client,
		options: options,
		keys:    newKeys(options.KeyPrefix),

		workflowQueue: workflowQueue,
		activityQueue: activityQueue,
	}

	// Preload scripts here. Usually redis-go attempts to execute them first, and if redis doesn't know
	// them, loads them. This doesn't work when using (transactional) pipelines, so eagerly load them on startup.
	cmds := map[string]*redis.StringCmd{
		"deleteInstanceCmd": deleteCmd.Load(ctx, rb.rdb),
		"addPayloadsCmd":    addPayloadsCmd.Load(ctx, rb.rdb),
	}
	for name, cmd := range cmds {
		// fmt.Println(name, cmd.Val())

		if cmd.Err() != nil {
			return nil, fmt.Errorf("loading redis script: %v %w", name, cmd.Err())
		}
	}

	// Load all Lua scripts
	cmdMapping := map[string]**redis.Script{
		"create_workflow_instance.lua": &createWorkflowInstanceCmd,
		"complete_workflow_task.lua":   &completeWorkflowTaskCmd,
		"schedule_future_events.lua":   &futureEventsCmd,
		"expire_workflow_instance.lua": &expireWorkflowInstanceCmd,
	}

	if err := loadScripts(ctx, rb.rdb, cmdMapping); err != nil {
		return nil, fmt.Errorf("loading Lua scripts: %w", err)
	}

	return rb, nil
}

func loadScripts(ctx context.Context, rdb redis.UniversalClient, cmdMapping map[string]**redis.Script) error {
	for scriptFile, cmd := range cmdMapping {
		scriptContent, err := fs.ReadFile(luaScripts, "scripts/"+scriptFile)
		if err != nil {
			return fmt.Errorf("reading Lua script %s: %w", scriptFile, err)
		}

		*cmd = redis.NewScript(string(scriptContent))

		if c := (*cmd).Load(ctx, rdb); c.Err() != nil {
			return fmt.Errorf("loading Lua script %s: %w", scriptFile, c.Err())
		}
	}

	return nil
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options *RedisOptions

	keys *keys

	workflowQueue *taskQueue[workflowData]
	activityQueue *taskQueue[activityData]
}

type workflowData struct{}

type activityData struct {
	Instance *core.WorkflowInstance `json:"instance,omitempty"`
	Queue    string                 `json:"queue,omitempty"`
	ID       string                 `json:"id,omitempty"`
	Event    *history.Event         `json:"event,omitempty"`
}

func (rb *redisBackend) Metrics() metrics.Client {
	return rb.options.Metrics.WithTags(metrics.Tags{metrickeys.Backend: "redis"})
}

func (rb *redisBackend) Tracer() trace.Tracer {
	return rb.options.TracerProvider.Tracer(backend.TracerName)
}

func (b *redisBackend) Options() *backend.Options {
	return b.options.Options
}

func (rb *redisBackend) Close() error {
	return rb.rdb.Close()
}

func (rb *redisBackend) FeatureSupported(feature backend.Feature) bool {
	switch feature {
	case backend.Feature_Expiration:
		return false
	}

	return true
}
