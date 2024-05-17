package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/signals"
	internal "github.com/cschleiden/go-workflows/internal/worker"
	"github.com/cschleiden/go-workflows/registry"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/cschleiden/go-workflows/workflow/executor"
)

type Worker struct {
	backend backend.Backend

	done chan struct{}
	wg   *sync.WaitGroup

	registry *registry.Registry

	workflowWorker *internal.Worker[backend.WorkflowTask, executor.ExecutionResult]
	activityWorker *internal.Worker[backend.ActivityTask, history.Event]
}

func New(backend backend.Backend, options *Options) *Worker {
	if options == nil {
		options = &DefaultOptions
	} else {
		if options.WorkflowExecutorCacheSize == 0 {
			options.WorkflowExecutorCacheSize = DefaultOptions.WorkflowExecutorCacheSize
		}

		if options.WorkflowExecutorCacheTTL == 0 {
			options.WorkflowExecutorCacheTTL = DefaultOptions.WorkflowExecutorCacheTTL
		}
	}

	registry := registry.New()

	// Register internal activities
	if err := registry.RegisterActivity(&signals.Activities{Signaler: client.New(backend)}); err != nil {
		panic(fmt.Errorf("registering internal activities: %w", err))
	}

	return &Worker{
		backend: backend,

		done: make(chan struct{}),
		wg:   &sync.WaitGroup{},

		workflowWorker: internal.NewWorkflowWorker(backend, registry, internal.WorkflowWorkerOptions{
			WorkerOptions: internal.WorkerOptions{
				Pollers:           options.WorkflowPollers,
				PollingInterval:   options.WorkflowPollingInterval,
				MaxParallelTasks:  options.MaxParallelWorkflowTasks,
				HeartbeatInterval: options.WorkflowHeartbeatInterval,
				Namespaces:        options.WorkflowNamespaces,
			},
			WorkflowExecutorCache:     options.WorkflowExecutorCache,
			WorkflowExecutorCacheSize: options.WorkflowExecutorCacheSize,
			WorkflowExecutorCacheTTL:  options.WorkflowExecutorCacheTTL,
		}),

		activityWorker: internal.NewActivityWorker(backend, registry, clock.New(), internal.WorkerOptions{
			Pollers:           options.ActivityPollers,
			PollingInterval:   options.ActivityPollingInterval,
			MaxParallelTasks:  options.MaxParallelActivityTasks,
			HeartbeatInterval: options.ActivityHeartbeatInterval,
			Namespaces:        options.ActivityNamespaces,
		}),

		registry: registry,
	}
}

// Start starts the worker.
//
// To stop the worker, cancel the context passed to Start. To wait for completion of the active
// tasks, call `WaitForCompletion`.
func (w *Worker) Start(ctx context.Context) error {
	if err := w.workflowWorker.Start(ctx); err != nil {
		return fmt.Errorf("starting workflow worker: %w", err)
	}

	if err := w.activityWorker.Start(ctx); err != nil {
		return fmt.Errorf("starting activity worker: %w", err)
	}

	return nil
}

// WaitForCompletion waits for all active tasks to complete.
func (w *Worker) WaitForCompletion() error {
	if err := w.workflowWorker.WaitForCompletion(); err != nil {
		return err
	}

	if err := w.activityWorker.WaitForCompletion(); err != nil {
		return err
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
