package task

import (
	"github.com/cschleiden/go-workflows/pkg/core"
	"github.com/cschleiden/go-workflows/pkg/history"
)

type Activity struct {
	ID string

	WorkflowInstance core.WorkflowInstance

	Event history.Event
}
