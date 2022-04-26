package history

import (
	"github.com/cschleiden/go-workflows/internal/core"
)

type SubWorkflowCancellationRequestedAttributes struct {
	SubWorkflowInstance *core.WorkflowInstance `json:"sub_workflow_instance,omitempty"`
}
