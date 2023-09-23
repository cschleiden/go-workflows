package history

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
)

type SubWorkflowScheduledAttributes struct {
	SubWorkflowInstance *core.WorkflowInstance `json:"sub_workflow_instance,omitempty"`

	Name string `json:"name,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`

	Metadata *metadata.WorkflowMetadata `json:"metadata,omitempty"`
}
