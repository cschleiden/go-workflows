package workflow

import "github.com/cschleiden/go-workflows/internal/sync"

type Channel[T any] = sync.Channel[T]

// NewChannel creates a new channel.
func NewChannel[T any]() Channel[T] {
	return sync.NewChannel[T]()
}

// NewBufferedChannel creates a new buffered channel with the given size.
func NewBufferedChannel[T any](size int) Channel[T] {
	return sync.NewBufferedChannel[T](size)
}
