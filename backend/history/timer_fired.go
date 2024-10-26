package history

import (
	"time"

	"github.com/cschleiden/go-workflows/backend/metadata"
)

type TimerFiredAttributes struct {
	ScheduledAt  time.Time                 `json:"scheduled_at,omitempty"`
	At           time.Time                 `json:"at,omitempty"`
	Name         string                    `json:"name,omitempty"`
	SpanMetadata metadata.WorkflowMetadata `json:"span_metadata,omitempty"`
}
