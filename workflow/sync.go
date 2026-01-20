package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

type (
	Context   = sync.Context
	WaitGroup = sync.WaitGroup
	ErrGroup  = sync.ErrGroup
)

// NewWaitGroup creates a new WaitGroup instance.
func NewWaitGroup() WaitGroup {
	return sync.NewWaitGroup()
}

// WithErrGroup creates a child context and errgroup for running workflow goroutines that return errors.
func WithErrGroup(ctx Context) (Context, ErrGroup) {
	return sync.WithErrGroup(ctx)
}

// Go spawns a workflow "goroutine".
func Go(ctx Context, f func(ctx Context)) {
	sync.Go(ctx, f)
}
