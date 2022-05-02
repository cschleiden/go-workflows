package sync

import (
	"errors"
)

type Future[T any] interface {
	// Get returns the value if set, blocks otherwise
	Get(ctx Context) (T, error)
}

type SettableFuture[T any] interface {
	Future[T]

	// Set stores the value
	Set(v T, err error) error
}

type FutureInternal[T any] interface {
	Future[T]

	Ready() bool
}

func NewFuture[T any]() SettableFuture[T] {
	return &future[T]{}
}

type future[T any] struct {
	hasValue bool
	v        T
	err      error
}

func (f *future[T]) Set(v T, err error) error {
	if f.hasValue {
		return errors.New("future already set")
	}

	f.v = v
	f.err = err
	f.hasValue = true

	return nil
}

func (f *future[T]) Get(ctx Context) (T, error) {
	for {
		cr := getCoState(ctx)

		if f.hasValue {
			cr.MadeProgress()

			if f.err != nil {
				return *new(T), f.err
			}

			return f.v, nil
		}

		cr.Yield()
	}
}

func (f *future[T]) Ready() bool {
	return f.hasValue
}
