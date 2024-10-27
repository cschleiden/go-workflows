package history

import "github.com/cschleiden/go-workflows/backend/payload"

type TraceStartedAttributes struct {
	SpanID payload.Payload `json:"spanID"`
}
