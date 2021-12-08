package task

import (
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Workflow struct {
	WorkflowInstance core.WorkflowInstance

	History []history.HistoryEvent
}
