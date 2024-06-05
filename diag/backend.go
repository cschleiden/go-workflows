package diag

import (
	"context"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
)

// json: serialization in this file needs to be kept in sync with client.ts in the web app

type WorkflowInstanceRef struct {
	Instance    *core.WorkflowInstance     `json:"instance,omitempty"`
	CreatedAt   time.Time                  `json:"created_at,omitempty"`
	CompletedAt *time.Time                 `json:"completed_at,omitempty"`
	State       core.WorkflowInstanceState `json:"state"`
	Queue       string                     `json:"queue"`
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

type WorkflowInstanceTree struct {
	*WorkflowInstanceRef

	WorkflowName string `json:"workflow_name,omitempty"`

	Children []*WorkflowInstanceTree `json:"children,omitempty"`
}

type Backend interface {
	backend.Backend

	GetWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) (*WorkflowInstanceRef, error)
	GetWorkflowInstances(ctx context.Context, afterInstanceID, afterExecutionID string, count int) ([]*WorkflowInstanceRef, error)
	GetWorkflowTree(ctx context.Context, instance *core.WorkflowInstance) (*WorkflowInstanceTree, error)
}
