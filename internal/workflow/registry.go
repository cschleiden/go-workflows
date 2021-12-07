package workflow

import "sync"

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

func (r *Registry) RegisterWorkflow(name string, workflow Workflow) {
	r.Lock()
	defer r.Unlock()

	r.workflowMap[name] = workflow
}

func (r *Registry) RegisterActivity(name string, activity Activity) {
	r.Lock()
	defer r.Unlock()

	r.activityMap[name] = activity
}

func (r *Registry) getWorkflow(name string) Workflow {
	r.Lock()
	defer r.Unlock()

	return r.workflowMap[name]
}

func (r *Registry) getActivity(name string) Activity {
	r.Lock()
	defer r.Unlock()

	return r.activityMap[name]
}
