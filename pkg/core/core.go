package core

type Promise interface {
	Get() (interface{}, error)
}

type WorkflowInstanceID string

type Workflow interface{}

type Activity interface{}
