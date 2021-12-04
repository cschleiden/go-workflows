package backend

import (
	"github.com/cschleiden/go-dt/pkg/core"
)

type Backend interface {
	CreateWorkflowInstance(message core.TaskMessage) error

	SendWorkflowInstanceMessage(message core.TaskMessage) error

	// GetWorkflowTask() (WorkItem, error)

	// GetActivityTask() (WorkItem, error)
}
