package workflow

import "github.com/cschleiden/go-workflows/internal/workflowerrors"

type (
	Error      = workflowerrors.Error
	PanicError = workflowerrors.PanicError
)

// NewError wraps the given error into a workflow error which will be automatically retried
func NewError(err error) error {
	return workflowerrors.FromError(err)
}

// NewPermanentError wraps the given error into a workflow error which will not be automatically retried
func NewPermanentError(err error) error {
	return workflowerrors.NewPermanentError(err)
}

// CanRetry returns true if the given error is retryable
func CanRetry(err error) bool {
	return workflowerrors.CanRetry(err)
}
