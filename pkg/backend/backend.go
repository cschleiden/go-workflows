package backend

import "github.com/cschleiden/go-dt/pkg/core"

type WorkItem struct {
	// TODO: Define work to be done
}

type Backend interface {
	CreateWorkflowInstance(core.WorkflowInstanceID, core.Workflow) error

	GetWorkflowTask() (WorkItem, error)

	GetActivityTask() (WorkItem, error)
}
