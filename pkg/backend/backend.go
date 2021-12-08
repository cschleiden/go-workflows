package backend

import (
	"context"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Backend interface {
	CreateWorkflowInstance(context.Context, core.TaskMessage) error

	GetWorkflowTask(context.Context) (*task.Workflow, error)

	CompleteWorkflowTask(context.Context, task.Workflow, []command.Command) error

	GetActivityTask(context.Context) (*task.Activity, error)

	CompleteActivityTask(context.Context, task.Activity, history.HistoryEvent) error
}
