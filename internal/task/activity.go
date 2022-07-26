package task

import (
	"github.com/ticctech/go-workflows/internal/core"
	"github.com/ticctech/go-workflows/internal/history"
)

type Activity struct {
	ID string

	WorkflowInstance *core.WorkflowInstance

	Metadata *core.WorkflowMetadata

	Event history.Event
}
