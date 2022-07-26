package history

import (
	"github.com/ticctech/go-workflows/internal/core"
)

type SubWorkflowCancellationRequestedAttributes struct {
	SubWorkflowInstance *core.WorkflowInstance `json:"sub_workflow_instance,omitempty"`
}
