package sync

import "context"

type Future interface {
	// Set stores the value and unblocks any waiting consumers
	Set(ctx context.Context, getter func(v interface{}) error)

	// Get returns the value if set, blocks otherwise
	Get(ctx context.Context, v interface{}) error
}

func NewFuture() Future {
	return &futureImpl{}
}

type futureImpl struct {
	fn func(v interface{}) error
}

func (f *futureImpl) Set(ctx context.Context, getter func(v interface{}) error) {
	f.fn = getter
}

func (f *futureImpl) Get(ctx context.Context, v interface{}) error {
	for {
		cr := getCoState(ctx)

		if f.fn != nil {
			cr.MadeProgress()

			return f.fn(v)
		}

		cr.Yield()
	}
}

func (f *futureImpl) Ready() bool {
	return f.fn != nil
}
