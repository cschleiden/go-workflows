package core

type WorkflowInstance struct {
	InstanceID  string `json:"instance_id,omitempty"`
	ExecutionID string `json:"execution_id,omitempty"`

	ParentInstanceID string `json:"parent_instance,omitempty"`
	ParentEventID    int64  `json:"parent_event_id,omitempty"`
}

func NewWorkflowInstance(instanceID, executionID string) *WorkflowInstance {
	return &WorkflowInstance{
		InstanceID:  instanceID,
		ExecutionID: executionID,
	}
}

func NewSubWorkflowInstance(instanceID, executionID string, parentInstanceID string, parentEventID int64) *WorkflowInstance {
	return &WorkflowInstance{
		InstanceID:       instanceID,
		ExecutionID:      executionID,
		ParentInstanceID: parentInstanceID,
		ParentEventID:    parentEventID,
	}
}

func (wi *WorkflowInstance) SubWorkflow() bool {
	return wi.ParentInstanceID != ""
}
