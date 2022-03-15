package workflow

type Future[T any] interface {
	// Get returns the value if set, blocks otherwise
	Get(ctx Context) (T, error)
}
