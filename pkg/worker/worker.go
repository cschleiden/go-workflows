package worker

import (
	"context"

	internal "github.com/cschleiden/go-dt/internal/worker"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
)

type WorkflowRegistry interface {
	RegisterWorkflow(w workflow.Workflow) error
}

type ActivityRegistry interface {
	RegisterActivity(a workflow.Activity) error
}

type Registry interface {
	WorkflowRegistry
	ActivityRegistry
}

type Worker interface {
	Registry

	// Start starts the worker
	Start(context.Context) error
}

type worker struct {
	backend backend.Backend

	registry *workflow.Registry

	workflowWorker internal.WorkflowWorker
	activityWorker internal.ActivityWorker

	workflows  map[string]interface{}
	activities map[string]interface{}
}

type Options = internal.Options

var DefaultWorkerOptions = internal.DefaultOptions

func New(backend backend.Backend, options *Options) Worker {
	if options == nil {
		options = &internal.DefaultOptions
	}

	registry := workflow.NewRegistry()

	return &worker{
		backend: backend,

		workflowWorker: internal.NewWorkflowWorker(backend, registry, options),
		activityWorker: internal.NewActivityWorker(backend, registry, options),

		registry: registry,

		workflows:  map[string]interface{}{},
		activities: map[string]interface{}{},
	}
}

func (w *worker) Start(ctx context.Context) error {
	w.workflowWorker.Start(ctx)
	w.activityWorker.Start(ctx)

	return nil
}

func (w *worker) RegisterWorkflow(wf workflow.Workflow) error {
	return w.registry.RegisterWorkflow(wf)
}

func (w *worker) RegisterActivity(a workflow.Activity) error {
	return w.registry.RegisterActivity(a)
}
