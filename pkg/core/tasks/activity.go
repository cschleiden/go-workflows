package tasks

import (
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

type ActivityTask struct {
	WorkflowInstance core.WorkflowInstance

	ID string

	// SequenceNumber uint64 // TODO: Required?

	Event history.HistoryEvent
}
