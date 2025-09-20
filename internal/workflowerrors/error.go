package workflowerrors

import (
	"encoding/json"
	"errors"
)

type Error struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`

	Permanent  bool   `json:"permanent,omitempty"`
	Cause      error  `json:"cause,omitempty"`
	Stacktrace string `json:"stacktrace,omitempty"`
}

func (we *Error) UnmarshalJSON(b []byte) error {
	type Alias Error
	a := &struct {
		Cause *Error `json:"cause,omitempty"`
		*Alias
	}{}

	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}

	*we = *(*Error)(a.Alias)
	we.Cause = a.Cause

	return nil
}

func (we *Error) Error() string {
	return we.Message
}

func (we *Error) Unwrap() error {
	if we == nil || we.Cause == (*Error)(nil) {
		return nil
	}

	return we.Cause
}

func (we *Error) Stack() string {
	return we.Stacktrace
}

var _ error = (*Error)(nil)

// FromError wraps the given error into a workflow error which can be persisted and restored
func FromError(err error) *Error {
	if err == nil {
		return nil
	}

	// If this is already a workflow error, just return it, do not wrap again
	if e, ok := err.(*Error); ok {
		return e
	}

	e := &Error{
		Type:    getErrorType(err),
		Message: err.Error(),
	}

	if stackTracer, ok := err.(interface{ Stack() string }); ok {
		e.Stacktrace = stackTracer.Stack()
	}

	if cause := errors.Unwrap(err); cause != nil {
		e.Cause = FromError(cause)
	}

	return e
}

// ToError attempts to convert the given workflow error into a regular error. It will create concrete errors for known error types
// and maintain the Error for unknown ones
func ToError(err *Error) error {
	if err == nil {
		return nil
	}

	e := *err

	switch err.Type {
	case getErrorType(&PanicError{}):
		return &PanicError{message: e.Message, stacktrace: e.Stacktrace}

	default:
		// Keep *Error
		return &e
	}
}

func NewPermanentError(err error) *Error {
	e := FromError(err)
	e.Permanent = true
	return e
}

// CanRetry returns true if the given error is retryable
func CanRetry(err error) bool {
	if e, ok := err.(*Error); ok {
		return !e.Permanent
	}

	// Retry errors by default
	return true
}
