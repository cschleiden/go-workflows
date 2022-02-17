package history

import "github.com/cschleiden/go-workflows/internal/payload"

type ExecutionStartedAttributes struct {
	Name string

	Inputs []payload.Payload
}
