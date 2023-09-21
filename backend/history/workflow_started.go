package history

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
)

type ExecutionStartedAttributes struct {
	Name string `json:"name,omitempty"`

	Metadata *metadata.WorkflowMetadata `json:"metadata,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`
}
