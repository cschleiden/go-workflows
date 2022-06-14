package history

import (
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type ExecutionStartedAttributes struct {
	Name string `json:"name,omitempty"`

	Metadata *core.WorkflowInstanceMetadata `json:"metadata,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`
}
