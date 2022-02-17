package workflow

import "github.com/cschleiden/go-workflows/internal/sync"

type CancelFunc = sync.CancelFunc

// WithCancel returns a copy of parent with a new Done channel. The returned
// context's Done channel is closed when the returned cancel function is called
// or when the parent context's Done channel is closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	return sync.WithCancel(parent)
}

func NewDisconnectedContext(ctx Context) Context {
	return sync.NewDisconnectedContext(ctx)
}
