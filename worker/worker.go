package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/signals"
	internal "github.com/cschleiden/go-workflows/internal/worker"
	workflowinternal "github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/workflow"
)

type Worker struct {
	backend backend.Backend

	done chan struct{}
	wg   *sync.WaitGroup

	registry *workflowinternal.Registry

	workflowWorker *internal.WorkflowWorker
	activityWorker *internal.ActivityWorker

	workflows  map[string]interface{}
	activities map[string]interface{}
}

type Options = internal.Options

var DefaultWorkerOptions = internal.DefaultOptions

func New(backend backend.Backend, options *Options) *Worker {
	if options == nil {
		options = &internal.DefaultOptions
	} else {
		if options.WorkflowExecutorCacheSize == 0 {
			options.WorkflowExecutorCacheSize = internal.DefaultOptions.WorkflowExecutorCacheSize
		}

		if options.WorkflowExecutorCacheTTL == 0 {
			options.WorkflowExecutorCacheTTL = internal.DefaultOptions.WorkflowExecutorCacheTTL
		}
	}

	registry := workflowinternal.NewRegistry()

	// Register internal activities
	registry.RegisterActivity(&signals.Activities{Signaler: client.New(backend)})

	return &Worker{
		backend: backend,

		done: make(chan struct{}),
		wg:   &sync.WaitGroup{},

		workflowWorker: internal.NewWorkflowWorker(backend, registry, options),
		activityWorker: internal.NewActivityWorker(backend, registry, clock.New(), options),

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

func (w *Worker) WaitForCompletion() error {
	if err := w.workflowWorker.WaitForCompletion(); err != nil {
		return err
	}

	if err := w.activityWorker.WaitForCompletion(); err != nil {
		return err
	}

	return nil
}

func (w *Worker) RegisterWorkflow(wf workflow.Workflow, opts ...RegisterOption) error {
	return w.registry.RegisterWorkflow(wf, registerOptions(opts).asInternalOptions()...)
}

func (w *Worker) RegisterActivity(a workflow.Activity, opts ...RegisterOption) error {
	return w.registry.RegisterActivity(a, registerOptions(opts).asInternalOptions()...)
}
