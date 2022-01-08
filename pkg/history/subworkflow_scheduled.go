package history

import "github.com/cschleiden/go-dt/internal/payload"

type SubWorkflowScheduledAttributes struct {
	InstanceID string

	Name string

	Version string

	Inputs []payload.Payload
}
