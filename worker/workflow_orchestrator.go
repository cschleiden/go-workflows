package worker

import (
	"context"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
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

	// Create copy of options without auto-enabling SingleWorkerMode
	orchestratorOptions := *options

	// Set default pollers to 1 for orchestrator mode (unless explicitly overridden)
	if orchestratorOptions.WorkflowPollers == DefaultOptions.WorkflowPollers {
		orchestratorOptions.WorkflowPollers = 1
	}
	if orchestratorOptions.ActivityPollers == DefaultOptions.ActivityPollers {
		orchestratorOptions.ActivityPollers = 1
	}

	// Create registry that will be shared between worker and orchestrator
	reg := registry.New()

	// Create a regular worker with the registry
	workflowWorker := newWorkflowWorker(backend, reg, &orchestratorOptions.WorkflowWorkerOptions)
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

// Start starts the worker.
func (o *WorkflowOrchestrator) Start(ctx context.Context) error {
	return o.worker.Start(ctx)
}

// WaitForCompletion waits for the worker to complete processing.
func (o *WorkflowOrchestrator) WaitForCompletion() error {
	return o.worker.WaitForCompletion()
}

// CreateWorkflowInstance creates a new workflow instance using the client.
func (o *WorkflowOrchestrator) CreateWorkflowInstance(ctx context.Context, options client.WorkflowInstanceOptions, wf workflow.Workflow, args ...any) (*workflow.Instance, error) {
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

// RegisterWorkflow registers a workflow with the orchestrator's registry.
func (o *WorkflowOrchestrator) RegisterWorkflow(wf workflow.Workflow, opts ...registry.RegisterOption) error {
	return o.worker.RegisterWorkflow(wf, opts...)
}

// RegisterActivity registers an activity with the orchestrator's registry.
func (o *WorkflowOrchestrator) RegisterActivity(a workflow.Activity, opts ...registry.RegisterOption) error {
	return o.worker.RegisterActivity(a, opts...)
}

// RemoveWorkflowInstance removes a workflow instance.
func (o *WorkflowOrchestrator) RemoveWorkflowInstance(ctx context.Context, instance *workflow.Instance) error {
	return o.Client.RemoveWorkflowInstance(ctx, instance)
}
