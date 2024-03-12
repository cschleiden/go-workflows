package history

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
)

type ActivityScheduledAttributes struct {
	Name string `json:"name,omitempty"`

	Attempt int `json:"attempt,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`

	Metadata *metadata.WorkflowMetadata `json:"metadata,omitempty"`
}
