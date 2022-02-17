package history

import "github.com/cschleiden/go-workflows/internal/payload"

type SubWorkflowScheduledAttributes struct {
	InstanceID string

	Name string

	Inputs []payload.Payload
}
