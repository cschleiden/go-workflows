package history

import "github.com/cschleiden/go-workflows/internal/core"

// WorkflowEvent is a event addressed for a specific workflow instance
type WorkflowEvent struct {
	WorkflowInstance *core.WorkflowInstance

	HistoryEvent Event
}
