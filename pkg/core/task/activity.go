package task

import (
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

type Activity struct {
	WorkflowInstance core.WorkflowInstance

	ID string

	Event history.Event
}
