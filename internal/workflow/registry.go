package workflow

import (
	"sync"

	"github.com/cschleiden/go-dt/internal/fn"
)

type Registry struct {
	sync.Mutex

	workflowMap map[string]Workflow
	activityMap map[string]Activity
}

func NewRegistry() *Registry {
	return &Registry{
		Mutex:       sync.Mutex{},
		workflowMap: make(map[string]Workflow),
		activityMap: make(map[string]Activity),
	}
}

func (r *Registry) RegisterWorkflow(workflow Workflow) error {
	r.Lock()
	defer r.Unlock()

	name := fn.Name(workflow)
	r.workflowMap[name] = workflow

	return nil
}

func (r *Registry) RegisterActivity(activity Activity) error {
	r.Lock()
	defer r.Unlock()

	name := fn.Name(activity)
	r.activityMap[name] = activity

	return nil
}

func (r *Registry) GetWorkflow(name string) Workflow {
	r.Lock()
	defer r.Unlock()

	return r.workflowMap[name]
}

func (r *Registry) GetActivity(name string) Activity {
	r.Lock()
	defer r.Unlock()

	return r.activityMap[name]
}
