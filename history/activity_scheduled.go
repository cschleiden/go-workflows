package history

import "github.com/cschleiden/go-workflows/internal/payload"

type ActivityScheduledAttributes struct {
	Name string

	Inputs []payload.Payload
}
