package task

import (
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/history"
)

type Activity struct {
	ID string

	WorkflowInstance core.WorkflowInstance

	Event history.Event
}
