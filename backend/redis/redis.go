package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/go-redis/redis/v8"
)

func NewRedisBackend(address, username, password string, db int) backend.Backend {
	return &redisBackend{
		client: *redis.NewClient(&redis.Options{
			Addr:     address,
			Username: username,
			Password: password,
			DB:       db,
		}),
	}
}

type redisBackend struct {
	client redis.Client
}

func (*redisBackend) CancelWorkflowInstance(ctx context.Context, instance core.WorkflowInstance) error {
	panic("unimplemented")
}

func (*redisBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, activityID string, event history.Event) error {
	panic("unimplemented")
}

func (*redisBackend) CompleteWorkflowTask(ctx context.Context, instance core.WorkflowInstance, state backend.WorkflowState, executedEvents []history.Event, activityEvents []history.Event, workflowEvents []history.WorkflowEvent) error {
	panic("unimplemented")
}

func (*redisBackend) CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error {
	panic("unimplemented")
}

func (*redisBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	panic("unimplemented")
}

func (*redisBackend) ExtendWorkflowTask(ctx context.Context, instance core.WorkflowInstance) error {
	panic("unimplemented")
}

func (*redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	panic("unimplemented")
}

func (*redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance core.WorkflowInstance) ([]history.Event, error) {
	panic("unimplemented")
}

func (*redisBackend) GetWorkflowInstanceState(ctx context.Context, instance core.WorkflowInstance) (backend.WorkflowState, error) {
	panic("unimplemented")
}

func (*redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	panic("unimplemented")
}

func (*redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	panic("unimplemented")
}
