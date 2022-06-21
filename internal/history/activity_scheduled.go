package history

import (
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type ActivityScheduledAttributes struct {
	Name string `json:"name,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`

	Metadata core.WorkflowMetadata `json:"metadata,omitempty"`
}
