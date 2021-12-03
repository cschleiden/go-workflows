package core

type Promise interface {
	Get() (interface{}, error)
}

type WorkflowInstanceID string
