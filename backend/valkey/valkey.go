package valkey

import (
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel/trace"
)

var _ backend.Backend = (*valkeyBackend)(nil)

//go:embed scripts
var luaScripts embed.FS

var (
	createWorkflowInstanceScript *valkey.Lua
	completeWorkflowTaskScript   *valkey.Lua
	completeActivityTaskScript   *valkey.Lua
	deleteInstanceScript         *valkey.Lua
	futureEventsScript           *valkey.Lua
	expireWorkflowInstanceScript *valkey.Lua
	cancelWorkflowInstanceScript *valkey.Lua
	signalWorkflowScript         *valkey.Lua
)

func NewValkeyBackend(client valkey.Client, opts ...BackendOption) (*valkeyBackend, error) {
	vopts := &Options{
		Options:      backend.ApplyOptions(),
		BlockTimeout: time.Second * 2,
	}

	for _, opt := range opts {
		opt(vopts)
	}

	workflowQueue, err := newTaskQueue[workflowData](vopts.KeyPrefix, "workflows", vopts.WorkerName)
	if err != nil {
		return nil, fmt.Errorf("creating workflow task queue: %w", err)
	}

	activityQueue, err := newTaskQueue[activityData](vopts.KeyPrefix, "activities", vopts.WorkerName)
	if err != nil {
		return nil, fmt.Errorf("creating activity task queue: %w", err)
	}

	vb := &valkeyBackend{
		client:        client,
		options:       vopts,
		keys:          newKeys(vopts.KeyPrefix),
		workflowQueue: workflowQueue,
		activityQueue: activityQueue,
	}

	// Load all Lua scripts
	scriptMapping := map[string]**valkey.Lua{
		"cancel_workflow_instance.lua": &cancelWorkflowInstanceScript,
		"complete_activity_task.lua":   &completeActivityTaskScript,
		"complete_workflow_task.lua":   &completeWorkflowTaskScript,
		"create_workflow_instance.lua": &createWorkflowInstanceScript,
		"delete_instance.lua":          &deleteInstanceScript,
		"expire_workflow_instance.lua": &expireWorkflowInstanceScript,
		"schedule_future_events.lua":   &futureEventsScript,
		"signal_workflow.lua":          &signalWorkflowScript,
	}

	if err := loadScripts(scriptMapping); err != nil {
		return nil, fmt.Errorf("loading Lua scripts: %w", err)
	}

	return vb, nil
}

func loadScripts(scriptMapping map[string]**valkey.Lua) error {
	for scriptFile, scriptVar := range scriptMapping {
		scriptContent, err := fs.ReadFile(luaScripts, "scripts/"+scriptFile)
		if err != nil {
			return fmt.Errorf("reading Lua script %s: %w", scriptFile, err)
		}

		*scriptVar = valkey.NewLuaScript(string(scriptContent))
	}

	return nil
}

type valkeyBackend struct {
	client        valkey.Client
	options       *Options
	keys          *keys
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

func (vb *valkeyBackend) Metrics() metrics.Client {
	return vb.options.Metrics.WithTags(metrics.Tags{metrickeys.Backend: "valkey"})
}

func (vb *valkeyBackend) Tracer() trace.Tracer {
	return vb.options.TracerProvider.Tracer(backend.TracerName)
}

func (vb *valkeyBackend) Options() *backend.Options {
	return vb.options.Options
}

func (vb *valkeyBackend) Close() error {
	vb.client.Close()
	return nil
}

func (vb *valkeyBackend) FeatureSupported(feature backend.Feature) bool {
	switch feature {
	case backend.Feature_Expiration:
		return false
	}

	return true
}
