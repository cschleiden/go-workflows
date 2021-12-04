package backend

import (
	"github.com/cschleiden/go-dt/internal/core"
	"github.com/cschleiden/go-dt/internal/workflow"
)

type WorkItem struct {
	// TODO: Define work to be done
}

type Backend interface {
	CreateWorkflowInstance(core.WorkflowInstanceID, workflow.Workflow) error

	GetWorkflowTask() (WorkItem, error)

	GetActivityTask() (WorkItem, error)
}
