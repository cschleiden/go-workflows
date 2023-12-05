package registry

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/fn"
	wf "github.com/cschleiden/go-workflows/workflow"
)

type Registry struct {
	sync.Mutex

	workflowMap map[string]wf.Workflow
	activityMap map[string]interface{}
}

// New creates a new registry instance.
func New() *Registry {
	return &Registry{
		workflowMap: make(map[string]wf.Workflow),
		activityMap: make(map[string]interface{}),
	}
}

type registerConfig struct {
	Name string
}

func (r *Registry) RegisterWorkflow(workflow wf.Workflow, opts ...RegisterOption) error {
	cfg := registerOptions(opts).applyRegisterOptions(registerConfig{})
	name := cfg.Name
	if name == "" {
		name = fn.Name(workflow)
	}

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

	r.Lock()
	defer r.Unlock()

	if _, ok := r.workflowMap[name]; ok {
		return &ErrWorkflowAlreadyRegistered{fmt.Sprintf("workflow with name %q already registered", name)}
	}
	r.workflowMap[name] = workflow

	return nil
}

func (r *Registry) RegisterActivity(activity wf.Activity, opts ...RegisterOption) error {
	cfg := registerOptions(opts).applyRegisterOptions(registerConfig{})

	t := reflect.TypeOf(activity)

	// Activities on struct
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		return r.registerActivitiesFromStruct(activity)
	}

	// Activity as function
	name := cfg.Name
	if name == "" {
		name = fn.Name(activity)
	}

	if err := checkActivity(reflect.TypeOf(activity)); err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()

	if _, ok := r.activityMap[name]; ok {
		return &ErrActivityAlreadyRegistered{fmt.Sprintf("activity with name %q already registered", name)}
	}
	r.activityMap[name] = activity

	return nil
}

func (r *Registry) registerActivitiesFromStruct(a interface{}) error {
	// Enumerate functions defined on a
	v := reflect.ValueOf(a)
	t := v.Type()

	r.Lock()
	defer r.Unlock()

	for i := 0; i < v.NumMethod(); i++ {
		mv := v.Method(i)
		mt := t.Method(i)

		// Ignore private methods
		if mt.PkgPath != "" {
			continue
		}

		if err := checkActivity(mt.Type); err != nil {
			return err
		}

		name := mt.Name
		r.activityMap[name] = mv.Interface()
	}

	return nil
}

func checkActivity(actType reflect.Type) error {
	if actType.Kind() != reflect.Func {
		return &ErrInvalidActivity{"activity not a func"}
	}

	if actType.NumOut() == 0 {
		return &ErrInvalidActivity{"activity must return error"}
	}

	errType := reflect.TypeOf((*error)(nil)).Elem()
	if !actType.Out(actType.NumOut() - 1).Implements(errType) {
		return &ErrInvalidWorkflow{"activity must return error as last return value"}
	}

	return nil
}

func (r *Registry) GetWorkflow(name string) (wf.Workflow, error) {
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

	return nil, fmt.Errorf("activity %s not found", name)
}
