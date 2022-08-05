package errors

import "fmt"

type PanicError struct {
	value any
}

func NewPanicError(v any) *PanicError {
	return &PanicError{v}
}

var _ error = (*PanicError)(nil)

func (pe *PanicError) Error() string {
	return fmt.Sprintf("%v", pe.value)
}
