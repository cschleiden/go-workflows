package history

import "github.com/cschleiden/go-workflows/internal/payload"

type SignalReceivedAttributes struct {
	Name string
	Arg  payload.Payload
}
