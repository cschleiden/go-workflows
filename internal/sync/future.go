package sync

type Future interface {
	// Set stores the value and unblocks any waiting consumers
	Set(getter func(v interface{}) error)

	// Get returns the value if set, blocks otherwise
	Get(v interface{}) error
}

func NewFuture(cr Coroutine) Future {
	return &futureImpl{
		cr: cr,
	}
}

type futureImpl struct {
	cr Coroutine
	fn func(v interface{}) error
}

func (f *futureImpl) Set(getter func(v interface{}) error) {
	f.fn = getter
}

func (f *futureImpl) Get(v interface{}) error {
	for {
		if f.fn != nil {
			return f.fn(v)
		}

		f.cr.Yield()
	}
}
