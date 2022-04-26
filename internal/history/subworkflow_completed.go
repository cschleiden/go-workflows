package history

import "github.com/cschleiden/go-workflows/internal/payload"

type SubWorkflowCompletedAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
