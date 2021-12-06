package core

type WorkflowInstance interface {
	GetInstanceID() string
	GetExecutionID() string
}

type workflowInstance struct {
	instanceID string

	executionID string
}

func NewWorkflowInstance(instanceID, executionID string) WorkflowInstance {
	return &workflowInstance{
		instanceID:  instanceID,
		executionID: executionID,
	}
}

func (wi *workflowInstance) GetInstanceID() string {
	return wi.instanceID
}

func (wi *workflowInstance) GetExecutionID() string {
	return wi.executionID
}
