package sync

import (
	"errors"

	"github.com/cschleiden/go-workflows/internal/converter"
)

type Future[T any] interface {
	// Get returns the value if set, blocks otherwise
	Get(ctx Context) (T, error)
}

type SettableFuture[T any] interface {
	Future[T]

	// Set stores the value and unblocks any waiting consumers
	Set(v T, err error) error
}

func NewFuture[T any]() SettableFuture[T] {
	return &future[T]{
		converter: converter.DefaultConverter,
	}
}

type future[T any] struct {
	hasValue  bool
	v         T
	err       error
	converter converter.Converter
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
				var zero T
				return zero, f.err
			}

			return f.v, nil
		}

		cr.Yield()
	}
}

func (f *future[T]) Ready() bool {
	return f.hasValue
}
