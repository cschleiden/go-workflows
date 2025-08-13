package worker

import (
	"context"
	"fmt"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/signals"
	internal "github.com/cschleiden/go-workflows/internal/worker"
	"github.com/cschleiden/go-workflows/internal/workflows"
	"github.com/cschleiden/go-workflows/registry"
	"github.com/cschleiden/go-workflows/workflow"
)

type Worker struct {
	backend backend.Backend

	registry *registry.Registry

	workers []worker
}

type worker interface {
	Start(context.Context) error
	WaitForCompletion() error
}

// New creates a worker that processes workflows and activities.
func New(backend backend.Backend, options *Options) *Worker {
	registry := registry.New()

	if options == nil {
		options = &DefaultOptions
	}

	workflowWorker := newWorkflowWorker(backend, registry, &options.WorkflowWorkerOptions)
	activityWorker := newActivityWorker(backend, registry, &options.ActivityWorkerOptions)

	// Register internal activities
	return newWorker(backend, registry, []worker{workflowWorker, activityWorker})
}

// NewWorkflowWorker creates a worker that only processes workflows.
func NewWorkflowWorker(backend backend.Backend, options *WorkflowWorkerOptions) *Worker {
	registry := registry.New()

	return newWorker(backend, registry, []worker{newWorkflowWorker(backend, registry, options)})
}

// NewActivityWorker creates a worker that only processes activities.
func NewActivityWorker(backend backend.Backend, options *ActivityWorkerOptions) *Worker {
	registry := registry.New()

	return newWorker(backend, registry, []worker{newActivityWorker(backend, registry, options)})
}

func newWorker(backend backend.Backend, registry *registry.Registry, workers []worker) *Worker {
	// Register system activites and workflows
	if err := registry.RegisterActivity(&signals.Activities{Signaler: client.New(backend)}); err != nil {
		panic(fmt.Errorf("registering internal activities: %w", err))
	}

	if err := registry.RegisterActivity(&workflows.Activities{Backend: backend}); err != nil {
		panic(fmt.Errorf("registering internal activities: %w", err))
	}

	if err := registry.RegisterWorkflow(workflows.ExpireWorkflowInstances); err != nil {
		panic(fmt.Errorf("registering internal workflow: %w", err))
	}

	return &Worker{
		backend: backend,

		workers:  workers,
		registry: registry,
	}
}

func newActivityWorker(backend backend.Backend, registry *registry.Registry, options *ActivityWorkerOptions) worker {
	if options == nil {
		options = &DefaultOptions.ActivityWorkerOptions
	}

	activityWorker := internal.NewActivityWorker(backend, registry, clock.New(), internal.WorkerOptions{
		Pollers:           options.ActivityPollers,
		PollingInterval:   options.ActivityPollingInterval,
		MaxParallelTasks:  options.MaxParallelActivityTasks,
		HeartbeatInterval: options.ActivityHeartbeatInterval,
		Queues:            options.ActivityQueues,
	})

	return activityWorker
}

func newWorkflowWorker(backend backend.Backend, registry *registry.Registry, options *WorkflowWorkerOptions) worker {
	if options == nil {
		options = &DefaultOptions.WorkflowWorkerOptions
	}

	workflowWorker := internal.NewWorkflowWorker(backend, registry, internal.WorkflowWorkerOptions{
		WorkerOptions: internal.WorkerOptions{
			Pollers:           options.WorkflowPollers,
			PollingInterval:   options.WorkflowPollingInterval,
			MaxParallelTasks:  options.MaxParallelWorkflowTasks,
			HeartbeatInterval: options.WorkflowHeartbeatInterval,
			Queues:            options.WorkflowQueues,
		},
		WorkflowExecutorCache:     options.WorkflowExecutorCache,
		WorkflowExecutorCacheSize: options.WorkflowExecutorCacheSize,
		WorkflowExecutorCacheTTL:  options.WorkflowExecutorCacheTTL,
	})

	return workflowWorker
}

// Start starts the worker.
//
// To stop the worker, cancel the context passed to Start. To wait for completion of the active
// tasks, call `WaitForCompletion`.
func (w *Worker) Start(ctx context.Context) error {
	for _, worker := range w.workers {
		if err := worker.Start(ctx); err != nil {
			return fmt.Errorf("starting worker: %w", err)
		}
	}

	return nil
}

// WaitForCompletion waits for all active tasks to complete.
func (w *Worker) WaitForCompletion() error {
	for _, worker := range w.workers {
		if err := worker.WaitForCompletion(); err != nil {
			return fmt.Errorf("waiting for worker completion: %w", err)
		}
	}

	return nil
}

// RegisterWorkflow registers a workflow with the worker's registry.
func (w *Worker) RegisterWorkflow(wf workflow.Workflow, opts ...registry.RegisterOption) error {
	return w.registry.RegisterWorkflow(wf, opts...)
}

// RegisterActivity registers an activity with the worker's registry.
func (w *Worker) RegisterActivity(a workflow.Activity, opts ...registry.RegisterOption) error {
	return w.registry.RegisterActivity(a, opts...)
}
