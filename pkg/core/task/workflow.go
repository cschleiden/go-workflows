package task

import (
	"github.com/cschleiden/go-workflows/pkg/core"
	"github.com/cschleiden/go-workflows/pkg/history"
)

type Kind int

const (
	_ Kind = iota
	Continuation
)

type Workflow struct {
	WorkflowInstance core.WorkflowInstance

	Kind Kind

	History   []history.Event
	NewEvents []history.Event
}
