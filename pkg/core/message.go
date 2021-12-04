package core

import (
	"github.com/cschleiden/go-dt/pkg/history"
)

type TaskMessage struct {
	HistoryEvent history.HistoryEvent

	WorkflowInstance WorkflowInstance
}
