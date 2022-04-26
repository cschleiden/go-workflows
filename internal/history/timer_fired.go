package history

import "time"

type TimerFiredAttributes struct {
	At time.Time `json:"at,omitempty"`
}
