package history

import "github.com/cschleiden/go-workflows/internal/workflowerrors"

type ActivityFailedAttributes struct {
	Error *workflowerrors.Error `json:"error,omitempty"`
}
