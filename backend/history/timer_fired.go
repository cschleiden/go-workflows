package history

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/tracing"
)

type TimerFiredAttributes struct {
	ScheduledAt  time.Time       `json:"scheduled_at,omitempty"`
	At           time.Time       `json:"at,omitempty"`
	Name         string          `json:"name,omitempty"`
	TraceContext tracing.Context `json:"span_metadata,omitempty"`
}
