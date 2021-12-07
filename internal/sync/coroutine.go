package sync

import (
	"context"
	"sync/atomic"
)

type key int

var coroutinesCtxKey key

type coState struct {
	blocking chan bool    // coroutine is going to be blocked
	unblock  chan bool    // channel to unblock block coroutine
	blocked  atomic.Value // coroutine is currently blocked
	finished atomic.Value // coroutine finished executing
}

type Coroutine interface {
	// Run starts the coroutine, must only be called once
	Run(ctx context.Context, fn func(ctx context.Context))
	WaitUntilBlocked()
	Continue()

	Blocked() bool
	Yield()
	Finished() bool
}

func NewCoroutine() Coroutine {
	return newState()
}

func newState() *coState {
	return &coState{
		blocking: make(chan bool, 1),
		unblock:  make(chan bool),
	}
}

func (s *coState) Run(ctx context.Context, fn func(ctx context.Context)) {
	ctx = withCoState(ctx, s)

	go func() {
		defer s.finish() // Ensure we always mark the coroutine as finished
		defer func() {
			// TODO: panic handling
		}()

		fn(ctx)
	}()
}

func (s *coState) finish() {
	s.finished.Store(true)
	s.blocking <- true
}

func (s *coState) WaitUntilBlocked() {
	<-s.blocking
}

func (s *coState) Finished() bool {
	v, ok := s.finished.Load().(bool)
	return ok && v
}

func (s *coState) Blocked() bool {
	v, ok := s.blocked.Load().(bool)
	return ok && v
}

func (s *coState) Yield() {
	s.blocked.Store(true)
	s.blocking <- true

	<-s.unblock

	s.blocked.Store(false)
}

func (s *coState) Continue() {
	s.unblock <- true

	// TODO: Add some timeout

	// Run until blocked (which is also true when finished)
	select {
	case <-s.blocking:
	}
}

func withCoState(ctx context.Context, s *coState) context.Context {
	return context.WithValue(ctx, coroutinesCtxKey, s)
}

func getCoState(ctx context.Context) *coState {
	s, ok := ctx.Value(coroutinesCtxKey).(*coState)
	if !ok {
		panic("could not find coroutine state")
	}

	return s
}
