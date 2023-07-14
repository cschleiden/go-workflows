package sync

type Future[T any] interface {
	// Get returns the value if set, blocks otherwise
	Get(ctx Context) (T, error)
}

type SettableFuture[T any] interface {
	Future[T]

	// Set stores the value and provided error
	Set(v T, err error)

	HasValue() bool
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

func (f *future[T]) Set(v T, err error) {
	f.v = v
	f.err = err
	f.hasValue = true
}

func (f *future[T]) HasValue() bool {
	return f.hasValue
}

func (f *future[T]) Get(ctx Context) (T, error) {
	for {
		cr := getCoState(ctx)

		if f.hasValue {
			cr.MadeProgress()

			if f.err != nil {
				return f.v, f.err
			}

			return f.v, nil
		}

		cr.Yield()
	}
}

func (f *future[T]) Ready() bool {
	return f.hasValue
}
