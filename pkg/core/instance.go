package core

type WorkflowInstance interface {
	GetInstanceID() string
	GetExecutionID() string

	ParentInstance() WorkflowInstance
	ParentEventID() int
	SubWorkflow() bool
}

type workflowInstance struct {
	instanceID  string
	executionID string

	parentInstance WorkflowInstance
	parentEventID  int
}

func NewWorkflowInstance(instanceID, executionID string) WorkflowInstance {
	return &workflowInstance{
		instanceID:  instanceID,
		executionID: executionID,
	}
}

func NewSubWorkflowInstance(instanceID, executionID string, parentInstance WorkflowInstance, parentEventID int) WorkflowInstance {
	return &workflowInstance{
		instanceID:     instanceID,
		executionID:    executionID,
		parentInstance: parentInstance,
		parentEventID:  parentEventID,
	}
}

func (wi *workflowInstance) GetInstanceID() string {
	return wi.instanceID
}

func (wi *workflowInstance) GetExecutionID() string {
	return wi.executionID
}

func (wi *workflowInstance) ParentInstance() WorkflowInstance {
	return wi.parentInstance
}

func (wi *workflowInstance) ParentEventID() int {
	return wi.parentEventID
}

func (wi *workflowInstance) SubWorkflow() bool {
	return wi.parentInstance != nil
}
