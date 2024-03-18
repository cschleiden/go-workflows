package workflow

import "github.com/cschleiden/go-workflows/internal/sync"

type Channel[T any] interface {
	// Send sends a value to the channel. If the channel is closed, this will panic.
	Send(ctx Context, v T)

	// SendNonblocking sends a value to the channel. This call is non-blocking and will return whether the
	// value could be sent. If the channel is closed, this will panic
	SendNonblocking(v T) (ok bool)

	// Receive receives a value from the channel.
	Receive(ctx Context) (v T, ok bool)

	// ReceiveNonBlocking tries to receives a value from the channel. This call is non-blocking and will return
	// whether the value could be returned.
	ReceiveNonBlocking() (v T, ok bool)

	// Close closes the channel. This will cause all future send operations to panic.
	Close()

	// Len returns the number of elements currently in the channel.
	Len() int
}

// NewChannel creates a new channel.
func NewChannel[T any]() Channel[T] {
	return sync.NewChannel[T]()
}

// NewBufferedChannel creates a new buffered channel with the given size.
func NewBufferedChannel[T any](size int) Channel[T] {
	return sync.NewBufferedChannel[T](size)
}
