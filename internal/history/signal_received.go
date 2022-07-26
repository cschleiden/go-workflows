package history

import "github.com/ticctech/go-workflows/internal/payload"

type SignalReceivedAttributes struct {
	Name string          `json:"name,omitempty"`
	Arg  payload.Payload `json:"arg,omitempty"`
}
