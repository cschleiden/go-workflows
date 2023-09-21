package history

import (
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
)

type ExecutionCompletedAttributes struct {
	Result payload.Payload       `json:"result,omitempty"`
	Error  *workflowerrors.Error `json:"error,omitempty"`
}
