package history

import "github.com/ticctech/go-workflows/internal/payload"

type SubWorkflowCompletedAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
