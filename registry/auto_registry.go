package registry

import (
	"fmt"
)

// AutoRegisteringRegistry extends the regular Registry with auto-registration capabilities
// It can be used as an alternative to context-based SingleWorkerMode
type AutoRegisteringRegistry struct {
	*Registry
}

// NewAutoRegistering creates a new auto-registering registry
func NewAutoRegistering() *AutoRegisteringRegistry {
	return &AutoRegisteringRegistry{
		Registry: New(),
	}
}

// GetWorkflowWithAutoRegister returns the workflow function for the given name.
// If the workflow is not found and wf is provided, it will attempt to register it.
func (r *AutoRegisteringRegistry) GetWorkflowWithAutoRegister(name string, wf interface{}) (interface{}, error) {
	v, err := r.Registry.GetWorkflow(name)

	// If not found, try to register the workflow
	if err != nil && wf != nil {
		if err := r.RegisterWorkflow(wf); err != nil {
			return nil, fmt.Errorf("auto-registering workflow %s: %w", name, err)
		}
		return r.Registry.GetWorkflow(name)
	}

	return v, err
}

// GetActivityWithAutoRegister returns the activity function for the given name.
// If the activity is not found and activity is provided, it will attempt to register it.
func (r *AutoRegisteringRegistry) GetActivityWithAutoRegister(name string, activity interface{}) (interface{}, error) {
	v, err := r.Registry.GetActivity(name)

	// If not found, try to register the activity
	if err != nil && activity != nil {
		if err := r.RegisterActivity(activity); err != nil {
			return nil, fmt.Errorf("auto-registering activity %s: %w", name, err)
		}
		return r.Registry.GetActivity(name)
	}

	return v, err
}
