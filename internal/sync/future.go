package sync

import (
	"sync/atomic"
)

type Future interface {
	// Set stores the value and unblocks any waiting consumers
	Set(v interface{})

	// Get returns the value if set, blocks otherwise
	Get() (interface{}, error)
}

func NewFuture(cr Coroutine) Future {
	return &futureImpl{
		cr: cr,
		c:  make(chan interface{}, 1),
		v:  atomic.Value{},
	}
}

type futureImpl struct {
	cr Coroutine
	c  chan interface{}
	v  atomic.Value
}

func (f *futureImpl) Set(v interface{}) {
	if f.v.Swap(v) != nil {
		panic("future value already set")
	}

	// Unblock blocking Gets
	f.c <- v
}

func (f *futureImpl) Get() (interface{}, error) {
	for {
		v := f.v.Load()
		if v != nil {
			return v, nil
		}

		f.cr.Yield()
	}
}
