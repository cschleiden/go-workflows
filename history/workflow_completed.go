package history

import "github.com/cschleiden/go-workflows/internal/payload"

type ExecutionCompletedAttributes struct {
	Result payload.Payload
	Error  string
}
