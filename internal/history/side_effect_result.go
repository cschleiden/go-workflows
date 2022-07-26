package history

import "github.com/ticctech/go-workflows/internal/payload"

type SideEffectResultAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
