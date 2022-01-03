package worker

import (
	"context"

	internal "github.com/cschleiden/go-dt/internal/worker"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
)

type WorkflowRegistry interface {
	RegisterWorkflow(name string, w workflow.Workflow) error
}

type ActivityRegistry interface {
	RegisterActivity(name string, a workflow.Activity) error
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

func NewWorker(backend backend.Backend) Worker {
	registry := workflow.NewRegistry()

	return &worker{
		backend: backend,

		workflowWorker: internal.NewWorkflowWorker(backend, registry),
		activityWorker: internal.NewActivityWorker(backend, registry),

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

func (w *worker) RegisterWorkflow(name string, wf workflow.Workflow) error {
	// TODO: Check for conflicts etc.

	w.registry.RegisterWorkflow(name, wf)

	return nil
}

func (w *worker) RegisterActivity(name string, a workflow.Activity) error {
	// TODO: Check for conflicts etc.

	w.registry.RegisterActivity(name, a)

	return nil
}
