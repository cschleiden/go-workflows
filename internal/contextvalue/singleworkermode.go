package contextvalue

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

type singleWorkerModeKey struct{}

// WithSingleWorkerMode sets the single worker mode flag in the context.
// When this flag is set, activities and workflows will be automatically registered when needed.
func WithSingleWorkerMode(ctx sync.Context) sync.Context {
	return sync.WithValue(ctx, singleWorkerModeKey{}, true)
}

// IsSingleWorkerMode checks if the context is in single worker mode.
func IsSingleWorkerMode(ctx sync.Context) bool {
	if v := ctx.Value(singleWorkerModeKey{}); v != nil {
		return v.(bool)
	}
	return false
}
