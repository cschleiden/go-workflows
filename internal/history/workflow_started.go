package history

import "github.com/cschleiden/go-workflows/internal/payload"

type ExecutionStartedAttributes struct {
	Name string `json:"name,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`
}
