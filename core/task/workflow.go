package task

import (
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/history"
)

type Kind int

const (
	_ Kind = iota
	Continuation
)

type Workflow struct {
	// WorkflowInstance is the workflow instance that this task is for
	WorkflowInstance core.WorkflowInstance

	// Kind defines what kind of task this is. A Continuation task only contains
	// new events and not the full history. By default the history is included.
	Kind Kind

	// History are the events that have been executed so far
	History []history.Event

	// NewEvents are new events since the last task execution
	NewEvents []history.Event
}
