package backend

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/converter"
	core "github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/metrics"
	"github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel/trace"
)

var ErrInstanceNotFound = errors.New("workflow instance not found")
var ErrInstanceAlreadyExists = errors.New("workflow instance already exists")
var ErrInstanceNotFinished = errors.New("workflow instance is not finished")

const TracerName = "go-workflow"

//go:generate mockery --name=Backend --inpackage
type Backend interface {
	// CreateWorkflowInstance creates a new workflow instance
	CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error

	// CancelWorkflowInstance cancels a running workflow instance
	CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, cancelEvent *history.Event) error

	// RemoveWorkflowInstance removes a workflow instance
	RemoveWorkflowInstance(ctx context.Context, instance *workflow.Instance) error

	// GetWorkflowInstanceState returns the state of the given workflow instance
	GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error)

	// GetWorkflowInstanceHistory returns the workflow history for the given instance. When lastSequenceID
	// is given, only events after that event are returned. Otherwise the full history is returned.
	GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]*history.Event, error)

	// SignalWorkflow signals a running workflow instance
	//
	// If the given instance does not exist, it will return an error
	SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error

	// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
	GetWorkflowTask(ctx context.Context) (*task.Workflow, error)

	// ExtendWorkflowTask extends the lock of a workflow task
	ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error

	// CompleteWorkflowTask checkpoints a workflow task retrieved using GetWorkflowTask
	//
	// This checkpoints the execution. events are new events from the last workflow execution
	// which will be added to the workflow instance history. workflowEvents are new events for the
	// completed or other workflow instances.
	CompleteWorkflowTask(
		ctx context.Context, task *task.Workflow, instance *workflow.Instance, state core.WorkflowInstanceState,
		executedEvents, activityEvents, timerEvents []*history.Event, workflowEvents []history.WorkflowEvent) error

	// GetActivityTask returns a pending activity task or nil if there are no pending activities
	GetActivityTask(ctx context.Context) (*task.Activity, error)

	// CompleteActivityTask completes an activity task retrieved using GetActivityTask
	CompleteActivityTask(ctx context.Context, instance *workflow.Instance, activityID string, event *history.Event) error

	// ExtendActivityTask extends the lock of an activity task
	ExtendActivityTask(ctx context.Context, activityID string) error

	// GetStats returns stats about the backend
	GetStats(ctx context.Context) (*Stats, error)

	// Logger returns the configured logger for the backend
	Logger() *slog.Logger

	// Tracer returns the configured trace provider for the backend
	Tracer() trace.Tracer

	// Metrics returns the configured metrics client for the backend
	Metrics() metrics.Client

	// Converter returns the configured converter for the backend
	Converter() converter.Converter

	// ContextPropagators returns the configured context propagators for the backend
	ContextPropagators() []contextpropagation.ContextPropagator
}
