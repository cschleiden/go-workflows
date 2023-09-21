package history

import "github.com/cschleiden/go-workflows/backend/payload"

type ActivityCompletedAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
