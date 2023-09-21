package core

type WorkflowInstance struct {
	InstanceID  string `json:"instance_id,omitempty"`
	ExecutionID string `json:"execution_id,omitempty"`

	Parent        *WorkflowInstance `json:"parent,omitempty"`
	ParentEventID int64             `json:"parent_event_id,omitempty"`
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
