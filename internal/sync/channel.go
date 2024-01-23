package sync

type Channel[T any] interface {
	Send(ctx Context, v T)

	SendNonblocking(v T) (ok bool)

	Receive(ctx Context) (v T, ok bool)

	ReceiveNonBlocking() (v T, ok bool)

	Close()
}

type Receiver[T any] struct {
	Receive func(v T, ok bool)
}

type ChannelInternal[T any] interface {
	Closed() bool

	ReceiveNonBlocking() (v T, ok bool)

	// AddReceiveCallback adds a callback that is called once when a value is sent to the channel. This is similar
	// to the blocking `Receive` method, but is not blocking a coroutine.
	AddReceiveCallback(rcb *Receiver[T])

	RemoveReceiveCallback(rcb *Receiver[T])
}

// Ensure channel implementation support internal interface
var _ ChannelInternal[struct{}] = (*channel[struct{}])(nil)

func NewChannel[T any]() Channel[T] {
	return &channel[T]{
		c: make([]T, 0),
	}
}

func NewBufferedChannel[T any](size int) Channel[T] {
	return &channel[T]{
		c:    make([]T, 0),
		size: size,
	}
}

type channel[T any] struct {
	c         []T
	receivers []*Receiver[T]
	senders   []func() T
	closed    bool
	size      int
}

func (c *channel[T]) Close() {
	c.closed = true

	// If there are still blocked senders, error
	if len(c.senders) > 0 {
		panic("send on closed channel")
	}

	// Drain buffered values
	for len(c.receivers) > 0 {
		r := c.receivers[0]
		c.receivers[0] = nil
		c.receivers = c.receivers[1:]

		// Send zero value to pending receiver
		var v T
		r.Receive(v, false)
	}
}

func (c *channel[T]) Send(ctx Context, v T) {
	cr := getCoState(ctx)

	addedSender := false
	sentValue := false

	for {
		if c.trySend(v) {
			cr.MadeProgress()
			return
		}

		if !addedSender {
			addedSender = true

			cb := func() T {
				sentValue = true
				return v
			}

			c.senders = append(c.senders, cb)
		}

		// No waiting receiver, yield
		cr.Yield()

		// Was our sender called while we yielded? If so, we can return
		if sentValue {
			cr.MadeProgress()
			return
		}
	}
}

func (c *channel[T]) SendNonblocking(v T) bool {
	return c.trySend(v)
}

func (c *channel[T]) Receive(ctx Context) (v T, ok bool) {
	cr := getCoState(ctx)

	addedListener := false
	receivedValue := false

	for {
		// Try to receive from buffered channel or blocked sender
		if v, ok, rok := c.tryReceive(); rok {
			cr.MadeProgress()
			return v, ok
		}

		// Register handler to receive value once
		if !addedListener {
			cb := &Receiver[T]{
				Receive: func(rv T, rok bool) {
					receivedValue = true
					v = rv
					ok = rok
				},
			}

			c.receivers = append(c.receivers, cb)
			addedListener = true
		}

		cr.Yield()

		// If we received a value via the callback, return
		if receivedValue {
			cr.MadeProgress()
			return v, ok
		}
	}
}

func (c *channel[T]) ReceiveNonBlocking() (T, bool) {
	if v, ok, rok := c.tryReceive(); rok {
		return v, ok
	}

	return *new(T), false
}

func (c *channel[T]) hasValue() bool {
	return len(c.c) > 0
}

func (c *channel[T]) canReceive() bool {
	return c.hasValue() || len(c.senders) > 0 || c.closed
}

func (c *channel[T]) canSend() bool {
	if c.closed {
		return false
	}

	return len(c.receivers) > 0 || c.hasCapacity()
}

func (c *channel[T]) trySend(v T) bool {
	// If closed, we can't send, panic.
	if c.closed {
		panic("channel closed")
	}

	// Are there any existing blocked receivers? If so, unblock the first one with
	// the value.
	if len(c.receivers) > 0 {
		r := c.receivers[0]
		c.receivers[0] = nil
		c.receivers = c.receivers[1:]

		r.Receive(v, true)

		return true
	}

	// No waiting receiver, if we have capacity try to add the value to the buffer
	if c.hasCapacity() {
		c.c = append(c.c, v)
		return true
	}

	// No receiver waiting and no capacity, we can't send.
	return false
}

func (c *channel[T]) tryReceive() (v T, ok bool, rok bool) {
	// If channel is buffered, return value if available
	if c.hasValue() {
		v = c.c[0]
		c.c = c.c[1:]

		return v, true, true
	}

	// If channel has been closed and no values in buffer (if buffered) return zero
	// element
	if c.closed {
		return *new(T), false, true
	}

	// Any blocked senders? If so, receive from the first one
	if len(c.senders) > 0 {
		s := c.senders[0]
		c.senders[0] = nil
		c.senders = c.senders[1:]

		return s(), true, true
	}

	// Could not receive value
	return v, ok, false
}

func (c *channel[T]) hasCapacity() bool {
	return len(c.c) < c.size
}

func (c *channel[T]) AddReceiveCallback(cb *Receiver[T]) {
	c.receivers = append(c.receivers, cb)
}

func (c *channel[T]) RemoveReceiveCallback(cb *Receiver[T]) {
	for i, r := range c.receivers {
		if r == cb {
			c.receivers[i] = nil
			c.receivers = append(c.receivers[:i], c.receivers[i+1:]...)
			return
		}
	}
}

func (c *channel[T]) Closed() bool {
	return c.closed
}
