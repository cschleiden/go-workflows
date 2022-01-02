package history

import "github.com/cschleiden/go-dt/internal/payload"

type SignalReceivedAttributes struct {
	Name string
	Arg  payload.Payload
}
