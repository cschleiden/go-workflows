package continueasnew

import (
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type Error struct {
	Metadata *core.WorkflowMetadata
	Inputs   []payload.Payload
}

var _ error = (*Error)(nil)

func (e *Error) Error() string {
	return "ContinueAsNew"
}

func NewError(metadata *core.WorkflowMetadata, inputs []payload.Payload) error {
	return &Error{
		Metadata: metadata,
		Inputs:   inputs,
	}
}
