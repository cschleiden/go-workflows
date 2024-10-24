package history

import "time"

type TimerFiredAttributes struct {
	ScheduledAt time.Time `json:"scheduled_at,omitempty"`
	At          time.Time `json:"at,omitempty"`
}
