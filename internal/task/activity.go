package task

import (
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
)

type Activity struct {
	ID string

	WorkflowInstance *core.WorkflowInstance

	Event *history.Event
}
