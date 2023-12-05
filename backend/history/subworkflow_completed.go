package history

import "github.com/cschleiden/go-workflows/backend/payload"

type SubWorkflowCompletedAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
