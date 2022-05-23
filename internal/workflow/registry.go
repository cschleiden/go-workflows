package workflow

import (
	"errors"
	"reflect"
	"sync"

	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/fn"
)

type Activity interface{}

type Registry struct {
	sync.Mutex

	workflowMap map[string]Workflow
	activityMap map[string]interface{}
}

func NewRegistry() *Registry {
	return &Registry{
		Mutex:       sync.Mutex{},
		workflowMap: make(map[string]Workflow),
		activityMap: make(map[string]interface{}),
	}
}

type ErrInvalidWorkflow struct {
	msg string
}

func (e *ErrInvalidWorkflow) Error() string {
	return e.msg
}

func (r *Registry) RegisterWorkflow(workflow Workflow) error {
	r.Lock()
	defer r.Unlock()

	wfType := reflect.TypeOf(workflow)
	if wfType.Kind() != reflect.Func {
		return &ErrInvalidWorkflow{"workflow is not a function"}
	}

	if wfType.NumIn() == 0 {
		return &ErrInvalidWorkflow{"workflow does not accept context parameter"}
	}

	if !args.IsOwnContext(wfType.In(0)) {
		return &ErrInvalidWorkflow{"workflow does not accept context as first parameter"}
	}

	if wfType.NumOut() == 0 {
		return &ErrInvalidWorkflow{"workflow must return error"}
	}

	if wfType.NumOut() > 2 {
		return &ErrInvalidWorkflow{"workflow must return at most two values"}
	}

	errType := reflect.TypeOf((*error)(nil)).Elem()
	if (wfType.NumOut() == 1 && !wfType.Out(0).Implements(errType)) ||
		(wfType.NumOut() == 2 && !wfType.Out(1).Implements(errType)) {
		return &ErrInvalidWorkflow{"workflow must return error as last return value"}
	}

	name := fn.Name(workflow)
	r.workflowMap[name] = workflow

	return nil
}

func (r *Registry) RegisterActivity(activity interface{}) error {
	r.Lock()
	defer r.Unlock()

	t := reflect.TypeOf(activity)

	// Activities on struct
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		return r.registerActivitiesFromStruct(activity)
	}

	// Activity as function
	name := fn.Name(activity)
	r.activityMap[name] = activity

	return nil
}

func (r *Registry) registerActivitiesFromStruct(a interface{}) error {
	// Enumerate functions defined on a
	v := reflect.ValueOf(a)
	t := v.Type()
	for i := 0; i < v.NumMethod(); i++ {
		mv := v.Method(i)
		mt := t.Method(i)

		// Ignore private methods
		if mt.PkgPath != "" {
			continue
		}

		name := mt.Name
		r.activityMap[name] = mv.Interface()
	}

	return nil
}

func (r *Registry) GetWorkflow(name string) (Workflow, error) {
	r.Lock()
	defer r.Unlock()

	if workflow, ok := r.workflowMap[name]; ok {
		return workflow, nil
	}

	return nil, errors.New("workflow not found")
}

func (r *Registry) GetActivity(name string) (interface{}, error) {
	r.Lock()
	defer r.Unlock()

	if activity, ok := r.activityMap[name]; ok {
		return activity, nil
	}

	return nil, errors.New("activity not found")
}
