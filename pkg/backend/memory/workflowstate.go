package memory

import (
	"time"

	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

type workflowStatus int

const (
	workflowStatusRunning workflowStatus = iota
	workflowStatusCompleted
)

type workflowState struct {
	Status workflowStatus

	Instance core.WorkflowInstance

	ParentInstance *core.WorkflowInstance

	History []history.Event

	PendingEvents []history.Event

	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (s *workflowState) getNewEvents() []history.Event {
	newEvents := make([]history.Event, 0)

	now := time.Now().UTC()

	for _, event := range s.PendingEvents {
		if event.VisibleAt == nil || event.VisibleAt.Before(now) {
			newEvents = append(newEvents, event)
		}
	}

	return newEvents
}
