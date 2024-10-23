package history

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
)

type SubWorkflowScheduledAttributes struct {
	SubWorkflowQueue core.Queue `json:"sub_workflow_queue,omitempty"`

	SubWorkflowInstance *core.WorkflowInstance `json:"sub_workflow_instance,omitempty"`

	Name string `json:"name,omitempty"`

	Inputs []payload.Payload `json:"inputs,omitempty"`

	Metadata *metadata.WorkflowMetadata `json:"metadata,omitempty"`

	WorkflowSpanID [8]byte `json:"workflow_span_id,omitempty"`
}
