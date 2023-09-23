package core

type WorkflowInstanceState int

const (
	WorkflowInstanceStateActive WorkflowInstanceState = iota
	WorkflowInstanceStateContinuedAsNew
	WorkflowInstanceStateFinished
)
