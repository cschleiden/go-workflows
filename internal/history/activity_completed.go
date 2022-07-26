package history

import "github.com/ticctech/go-workflows/internal/payload"

type ActivityCompletedAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
