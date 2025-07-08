package workflow

import (
	"github.com/cschleiden/go-workflows/internal/contextvalue"
)

// WithSingleWorkerMode sets the single worker mode flag in the context.
// When in single worker mode, workflows and activities are automatically registered as needed.
func WithSingleWorkerMode(ctx Context) Context {
	return contextvalue.WithSingleWorkerMode(ctx)
}

// IsSingleWorkerMode checks if the context is in single worker mode.
func IsSingleWorkerMode(ctx Context) bool {
	return contextvalue.IsSingleWorkerMode(ctx)
}
