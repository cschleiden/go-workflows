package tasks

import (
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

type WorkflowTask struct {
	WorkflowInstance core.WorkflowInstance

	History []history.HistoryEvent
}
