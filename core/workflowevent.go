package core

import (
	"github.com/cschleiden/go-workflows/history"
)

// WorkflowEvent is a event addressed for a specific workflow instance
type WorkflowEvent struct {
	WorkflowInstance WorkflowInstance

	HistoryEvent history.Event
}
