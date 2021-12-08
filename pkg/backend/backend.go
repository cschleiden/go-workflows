package backend

import (
	"context"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
)

type Backend interface {
	CreateWorkflowInstance(context.Context, core.TaskMessage) error

	GetWorkflowTask(context.Context) (*tasks.WorkflowTask, error)

	CompleteWorkflowTask(context.Context, tasks.WorkflowTask, []command.Command) error

	GetActivityTask(context.Context) (*tasks.ActivityTask, error)

	CompleteActivityTask(context.Context) error
}
