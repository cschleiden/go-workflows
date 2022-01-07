package sync

import "github.com/cschleiden/go-dt/internal/converter"

type Future interface {
	// Set stores the value and unblocks any waiting consumers
	Set(v interface{})

	// Get returns the value if set, blocks otherwise
	Get(ctx Context, vptr interface{}) error
}

func NewFuture() Future {
	return &futureImpl{
		converter: converter.DefaultConverter,
	}
}

type futureImpl struct {
	hasValue  bool
	v         interface{}
	converter converter.Converter
}

func (f *futureImpl) Set(v interface{}) {
	f.v = v
	f.hasValue = true
}

func (f *futureImpl) Get(ctx Context, vptr interface{}) error {
	for {
		cr := getCoState(ctx)

		if f.hasValue {
			cr.MadeProgress()

			if vptr != nil {
				converter.AssignValue(f.converter, f.v, vptr)
			}

			return nil
		}

		cr.Yield()
	}
}

func (f *futureImpl) Ready() bool {
	return f.hasValue
}
