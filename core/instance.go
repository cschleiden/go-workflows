package core

type WorkflowInstance struct {
	// InstanceID is the ID of the workflow instance.
	InstanceID string `json:"instance_id,omitempty"`

	// ExecutionID is the ID of the current execution of the workflow instance.
	ExecutionID string `json:"execution_id,omitempty"`

	// Parent refers to the parent workflow instance if this instance is a sub-workflow.
	Parent *WorkflowInstance `json:"parent,omitempty"`

	// ParentEventID is the ID of the event in the parent workflow that started this sub-workflow.
	ParentEventID int64 `json:"parent_event_id,omitempty"`
}

func NewWorkflowInstance(instanceID, executionID string) *WorkflowInstance {
	return &WorkflowInstance{
		InstanceID:  instanceID,
		ExecutionID: executionID,
	}
}

func NewSubWorkflowInstance(instanceID, executionID string, parentInstance *WorkflowInstance, parentEventID int64) *WorkflowInstance {
	return &WorkflowInstance{
		InstanceID:    instanceID,
		ExecutionID:   executionID,
		Parent:        parentInstance,
		ParentEventID: parentEventID,
	}
}

func (wi *WorkflowInstance) SubWorkflow() bool {
	return wi.Parent != nil
}
