package history

import "github.com/cschleiden/go-dt/internal/payload"

type ActivityScheduledAttributes struct {
	Name string

	Version string

	Inputs []payload.Payload
}
