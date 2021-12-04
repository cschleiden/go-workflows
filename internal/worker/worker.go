package worker

import "github.com/cschleiden/go-dt/pkg/backend"

type WorkflowRegistry interface {
	RegisterWorkflow(name string, w interface{}) error
}

type ActivityRegistry interface {
	RegisterActivity(name string, a interface{}) error
}

type Worker interface {
	Registry

	Start() error
	Stop() error
}

type Registry interface {
	WorkflowRegistry
	ActivityRegistry
}

type worker struct {
	backend    backend.Backend
	workflows  map[string]interface{}
	activities map[string]interface{}
}

func NewWorker(backend backend.Backend) Worker {
	return &worker{
		backend:    backend,
		workflows:  map[string]interface{}{},
		activities: map[string]interface{}{},
	}
}

func (w *worker) Start() error {
	return nil
}

func (w *worker) Stop() error {
	return nil
}

func (w *worker) RegisterWorkflow(name string, wf interface{}) error {
	// TODO: Check for conflicts etc.

	w.workflows[name] = wf

	return nil
}

func (w *worker) RegisterActivity(name string, a interface{}) error {
	// TODO: Check for conflicts etc.

	w.activities[name] = a

	return nil
}
