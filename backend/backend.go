package backend

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/trace"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
)

var (
	ErrInstanceNotFound      = errors.New("workflow instance not found")
	ErrInstanceAlreadyExists = errors.New("workflow instance already exists")
	ErrInstanceNotFinished   = errors.New("workflow instance is not finished")
)

type ErrNotSupported struct {
	Message string
}

func (e ErrNotSupported) Error() string {
	return fmt.Sprintf("not supported: %s", e.Message)
}

const TracerName = "go-workflow"

//go:generate mockery --name=Backend --inpackage
type Backend interface {
	// CreateWorkflowInstance creates a new workflow instance
	CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error

	// CancelWorkflowInstance cancels a running workflow instance
	CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, cancelEvent *history.Event) error

	// RemoveWorkflowInstance removes a workflow instance
	RemoveWorkflowInstance(ctx context.Context, instance *workflow.Instance) error

	// RemoveWorkflowInstances removes multiple workflow instances
	RemoveWorkflowInstances(ctx context.Context, options ...RemovalOption) error

	// GetWorkflowInstanceState returns the state of the given workflow instance
	GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error)

	// GetWorkflowInstanceHistory returns the workflow history for the given instance. When lastSequenceID
	// is given, only events after that event are returned. Otherwise the full history is returned.
	GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]*history.Event, error)

	// SignalWorkflow signals a running workflow instance
	//
	// If the given instance does not exist, it will return an error
	SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error

	// PrepareWorkflowQueues prepares workflow queues for later consumption using this backend instane
	PrepareWorkflowQueues(ctx context.Context, queues []workflow.Queue) error

	// PrepareActivityQueues prepares activity queues for later consumption using this backend instance
	PrepareActivityQueues(ctx context.Context, queues []workflow.Queue) error

	// GetWorkflowTask returns a pending workflow task or nil if there are no pending workflow executions
	GetWorkflowTask(ctx context.Context, queues []workflow.Queue) (*WorkflowTask, error)

	// ExtendWorkflowTask extends the lock of a workflow task
	ExtendWorkflowTask(ctx context.Context, task *WorkflowTask) error

	// CompleteWorkflowTask checkpoints a workflow task retrieved using GetWorkflowTask
	//
	// This checkpoints the execution. events are new events from the last workflow execution
	// which will be added to the workflow instance history. workflowEvents are new events for the
	// completed or other workflow instances.
	CompleteWorkflowTask(
		ctx context.Context, task *WorkflowTask, state core.WorkflowInstanceState,
		executedEvents, activityEvents, timerEvents []*history.Event, workflowEvents []*history.WorkflowEvent) error

	// GetActivityTask returns a pending activity task or nil if there are no pending activities
	GetActivityTask(ctx context.Context, queues []workflow.Queue) (*ActivityTask, error)

	// ExtendActivityTask extends the lock of an activity task
	ExtendActivityTask(ctx context.Context, task *ActivityTask) error

	// CompleteActivityTask completes an activity task retrieved using GetActivityTask
	CompleteActivityTask(ctx context.Context, task *ActivityTask, result *history.Event) error

	// GetStats returns stats about the backend
	GetStats(ctx context.Context) (*Stats, error)

	// Tracer returns the configured trace provider for the backend
	Tracer() trace.Tracer

	// Metrics returns the configured metrics client for the backend
	Metrics() metrics.Client

	// Options returns the configured options for the backend
	Options() *Options

	// Close closes any underlying resources
	Close() error

	// FeatureSupported returns true if the given feature is supported by the backend
	FeatureSupported(feature Feature) bool
}
