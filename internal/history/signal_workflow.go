package history

import "github.com/cschleiden/go-workflows/internal/payload"

type SignalWorkflowAttributes struct {
	Name string          `json:"name,omitempty"`
	Arg  payload.Payload `json:"arg,omitempty"`
}
