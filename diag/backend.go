package diag

import (
	"context"
	"time"

	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/internal/core"
)

// json: serialization in this file needs to be kept in sync with client.ts in the web app

type WorkflowInstanceRef struct {
	Instance    *core.WorkflowInstance `json:"instance,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	State       backend.WorkflowState  `json:"state"`
}

type Event struct {
	ID              string      `json:"id,omitempty"`
	SequenceID      int64       `json:"sequence_id,omitempty"`
	Type            string      `json:"type,omitempty"`
	Timestamp       time.Time   `json:"timestamp,omitempty"`
	ScheduleEventID int64       `json:"schedule_event_id,omitempty"`
	Attributes      interface{} `json:"attributes,omitempty"`
	VisibleAt       *time.Time  `json:"visible_at,omitempty"`
}

type WorkflowInstanceInfo struct {
	*WorkflowInstanceRef

	History []*Event `json:"history,omitempty"`
}

type Backend interface {
	backend.Backend

	GetWorkflowInstance(ctx context.Context, instanceID string) (*WorkflowInstanceRef, error)
	GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*WorkflowInstanceRef, error)
}
