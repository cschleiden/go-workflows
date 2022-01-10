package task

import (
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Activity struct {
	ID string

	WorkflowInstance core.WorkflowInstance

	Event history.Event
}
