package history

import "github.com/cschleiden/go-dt/internal/payload"

type ExecutionCompletedAttributes struct {
	Result payload.Payload
	Error  string
}
