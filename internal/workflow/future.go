package workflow

import (
	"context"
	"sync/atomic"
)

type Future interface {
	// Set stores the value and unblocks any waiting consumers
	Set(v interface{})

	// Get returns the value if set, blocks otherwise
	Get(ctx context.Context) (interface{}, error)
}

type futureImpl struct {
	c chan interface{}
	v atomic.Value
}

func newFuture() Future {
	return &futureImpl{
		c: make(chan interface{}, 1),
		v: atomic.Value{},
	}
}

func (f *futureImpl) Set(v interface{}) {
	if f.v.Swap(v) != nil {
		panic("future value already set")
	}

	// Unblock blocking Gets
	f.c <- v
}

func (f *futureImpl) Get(ctx context.Context) (interface{}, error) {
	for {
		v := f.v.Load()
		if v != nil {
			return v, nil
		}

		s := getCoState(ctx)
		s.yield()
	}
}
