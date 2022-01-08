package core

import (
	"github.com/cschleiden/go-dt/pkg/history"
)

type TaskMessage struct {
	WorkflowInstance WorkflowInstance

	HistoryEvent history.HistoryEvent
}
