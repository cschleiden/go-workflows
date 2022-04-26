package history

import "github.com/cschleiden/go-workflows/internal/payload"

type SideEffectResultAttributes struct {
	Result payload.Payload `json:"result,omitempty"`
}
