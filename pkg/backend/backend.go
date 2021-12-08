package backend

import (
	"context"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Backend interface {
	CreateWorkflowInstance(context.Context, core.TaskMessage) error

	GetWorkflowTask(context.Context) (*tasks.Workflow, error)

	CompleteWorkflowTask(context.Context, tasks.Workflow, []command.Command) error

	GetActivityTask(context.Context) (*tasks.Activity, error)

	CompleteActivityTask(context.Context, tasks.Activity, history.HistoryEvent) error
}
