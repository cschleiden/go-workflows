package backend

import (
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/core"
)

// WorkflowTask represents work for one workflow execution slice.
type WorkflowTask struct {
	// ID is an identifier for this task. It's set by the backend
	ID string

	// WorkflowInstance is the workflow instance that this task is for
	WorkflowInstance *core.WorkflowInstance

	WorkflowInstanceState core.WorkflowInstanceState

	Metadata *metadata.WorkflowMetadata

	// LastSequenceID is the sequence ID of the newest event in the workflow instances's history
	LastSequenceID int64

	// NewEvents are new events since the last task execution
	NewEvents []*history.Event

	// Backend specific data, only the producer of the task should rely on this.
	CustomData any
}

// ActivityTask represents one activity execution.
type ActivityTask struct {
	ID string

	WorkflowInstance *core.WorkflowInstance

	Event *history.Event
}
