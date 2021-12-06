package backend

import (
	"context"

	"github.com/cschleiden/go-dt/pkg/core"
)

type Backend interface {
	CreateWorkflowInstance(context.Context, core.TaskMessage) error

	SendWorkflowInstanceMessage(context.Context, core.TaskMessage) error

	// GetWorkflowTask() (WorkItem, error)

	// GetActivityTask() (WorkItem, error)
}
