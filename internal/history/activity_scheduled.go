package history

import (
	"github.com/ticctech/go-workflows/internal/core"
	"github.com/ticctech/go-workflows/internal/payload"
)

type ActivityScheduledAttributes struct {
	Name string `json:"name,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`

	Metadata core.WorkflowMetadata `json:"metadata,omitempty"`
}
