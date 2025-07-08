package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/registry"
	"github.com/cschleiden/go-workflows/workflow"
)

// WorkflowOrchestrator combines a worker and client into a single entity.
// It orchestrates the entire workflow lifecycle, from creation to execution.
type WorkflowOrchestrator struct {
	worker   *Worker
	Client   *client.Client // Exposed for direct access to GetWorkflowResult
	registry *registry.Registry
}

// NewWorkflowOrchestrator creates a new orchestrator with client capabilities and optional registration.
func NewWorkflowOrchestrator(backend backend.Backend, options *Options) *WorkflowOrchestrator {
	if options == nil {
		options = &DefaultOptions
	}

	// Enable SingleWorkerMode automatically for the orchestrator
	orchestratorOptions := *options
	orchestratorOptions.SingleWorkerMode = true

	// Create registry that will be shared between worker and orchestrator
	reg := registry.New()

	// Create a regular worker with the registry and SingleWorkerMode enabled
	workflowWorker := newWorkflowWorker(backend, reg, &orchestratorOptions.WorkflowWorkerOptions, orchestratorOptions.SingleWorkerMode)
	activityWorker := newActivityWorker(backend, reg, &orchestratorOptions.ActivityWorkerOptions)
	w := newWorker(backend, reg, []worker{workflowWorker, activityWorker})
	c := client.New(backend)

	// Create orchestrator that combines both
	orchestrator := &WorkflowOrchestrator{
		worker:   w,
		Client:   c,
		registry: reg,
	}

	return orchestrator
}

// Start starts the worker with single worker mode enabled.
func (o *WorkflowOrchestrator) Start(ctx context.Context) error {
	return o.worker.Start(ctx)
}

// WaitForCompletion waits for the worker to complete processing.
func (o *WorkflowOrchestrator) WaitForCompletion() error {
	return o.worker.WaitForCompletion()
}

// CreateWorkflowInstance creates a new workflow instance using the client.
// Automatically registers the workflow if it's not already registered.
func (o *WorkflowOrchestrator) CreateWorkflowInstance(ctx context.Context, options client.WorkflowInstanceOptions, wf workflow.Workflow, args ...any) (*workflow.Instance, error) {
	// Check if the workflow is a function (not a string name) and register it if needed
	if _, ok := wf.(string); !ok {
		// It's a function reference, try to register it if not already registered
		name := fn.Name(wf)
		_, err := o.registry.GetWorkflow(name)
		if err != nil {
			// Workflow not found in registry, register it directly
			if err := o.worker.RegisterWorkflow(wf); err != nil {
				return nil, fmt.Errorf("auto-registering workflow %s: %w", name, err)
			}
		}
	}

	return o.Client.CreateWorkflowInstance(ctx, options, wf, args...)
}

// WaitForWorkflowInstance waits for a workflow instance to complete.
func (o *WorkflowOrchestrator) WaitForWorkflowInstance(ctx context.Context, instance *workflow.Instance, timeout time.Duration) error {
	return o.Client.WaitForWorkflowInstance(ctx, instance, timeout)
}

// Note: Use client.GetWorkflowResult directly with the embedded client:
// result, err := client.GetWorkflowResult[ResultType](ctx, orchestrator.client, instance, timeout)

// SignalWorkflow signals a workflow instance.
func (o *WorkflowOrchestrator) SignalWorkflow(ctx context.Context, instanceID string, name string, arg any) error {
	return o.Client.SignalWorkflow(ctx, instanceID, name, arg)
}

// RemoveWorkflowInstance removes a workflow instance.
func (o *WorkflowOrchestrator) RemoveWorkflowInstance(ctx context.Context, instance *workflow.Instance) error {
	return o.Client.RemoveWorkflowInstance(ctx, instance)
}
