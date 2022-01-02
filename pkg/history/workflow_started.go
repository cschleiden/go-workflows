package history

import "github.com/cschleiden/go-dt/internal/payload"

type ExecutionStartedAttributes struct {
	Name string

	Version string

	Inputs []payload.Payload
}
