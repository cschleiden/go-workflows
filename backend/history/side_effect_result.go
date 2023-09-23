package history

import "github.com/cschleiden/go-workflows/backend/payload"

type SideEffectResultAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
