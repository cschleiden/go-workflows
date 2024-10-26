package history

import "time"

type TimerScheduledAttributes struct {
	At   time.Time `json:"at,omitempty"`
	Name string    `json:"name,omitempty"`
}
