package sync

import (
	"github.com/cschleiden/go-dt/internal/converter"
)

// TODO: Support cancellation??
type Channel interface {
	Send(ctx Context, v interface{})

	SendNonblocking(ctx Context, v interface{}) (ok bool)

	Receive(ctx Context, vptr interface{}) (more bool)

	ReceiveNonblocking(ctx Context, vptr interface{}) (more bool)

	Close()
}

type ChannelInternal interface {
}

func NewChannel() Channel {
	return &channel{
		c:         make([]interface{}, 0),
		converter: converter.DefaultConverter,
	}
}

func NewBufferedChannel(size int) Channel {
	return &channel{
		c:         make([]interface{}, 0, size),
		size:      size,
		converter: converter.DefaultConverter,
	}
}

type channel struct {
	c         []interface{}
	receivers []func(interface{})
	senders   []func() interface{}
	closed    bool
	size      int
	converter converter.Converter
}

func (c *channel) Close() {
	c.closed = true

	// TODO: Wake up all blocked sends

	for len(c.receivers) > 0 {
		r := c.receivers[0]
		c.receivers[0] = nil
		c.receivers = c.receivers[1:]

		r(nil)
	}
}

func (c *channel) Send(ctx Context, v interface{}) {
	addedSender := false
	sentValue := false

	for {
		if c.trySend(v) {
			return
		}

		if !addedSender {
			addedSender = true

			cb := func() interface{} {
				sentValue = true
				return v
			}

			c.senders = append(c.senders, cb)
		}

		cr := getCoState(ctx)
		cr.Yield()

		if sentValue {
			return
		}
	}
}

func (c *channel) SendNonblocking(ctx Context, v interface{}) bool {
	return c.trySend(v)
}

func (c *channel) Receive(ctx Context, vptr interface{}) (more bool) {
	cr := getCoState(ctx)

	addedListener := false
	receivedValue := false

	for {
		// Try to receive from buffered channel or blocked sender
		if c.tryReceive(vptr) {
			return !c.closed
		}

		// Register handler to receive value once
		if !addedListener {
			cb := func(v interface{}) {
				receivedValue = true

				converter.AssignValue(c.converter, v, vptr)
			}

			c.receivers = append(c.receivers, cb)
			addedListener = true
		}

		cr.Yield()

		// If we received a value via the callback, return
		if receivedValue {
			return !c.closed
		}
	}
}

func (c *channel) ReceiveNonblocking(ctx Context, vptr interface{}) (ok bool) {
	return c.tryReceive(vptr)
}

func (c *channel) hasValue() bool {
	return len(c.c) > 0
}

func (c *channel) canReceive() bool {
	return c.hasValue() || len(c.senders) > 0 || c.closed
}

func (c *channel) trySend(v interface{}) bool {
	// If closed, we can't send, exit.
	if c.closed {
		panic("channel closed")
	}

	// Are there any existing blocked receivers? If so, unblock the first one with
	// the value.
	if len(c.receivers) > 0 {
		r := c.receivers[0]
		c.receivers[0] = nil
		c.receivers = c.receivers[1:]
		r(v)
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

func (c *channel) tryReceive(vptr interface{}) bool {
	// If channel is buffered, return value if available
	if c.hasValue() {
		v := c.c[0]
		c.c = c.c[1:]

		converter.AssignValue(c.converter, v, vptr)
		return true
	}

	// If channel has been closed and no values in buffer (if buffered) return zero
	// element
	if c.closed {
		converter.AssignValue(c.converter, nil, vptr)
		return true
	}

	if len(c.senders) > 0 {
		s := c.senders[0]
		c.senders[0] = nil
		c.senders = c.senders[1:]

		// TODO: Or pass vtpr here?
		v := s()

		converter.AssignValue(c.converter, v, vptr)

		return true
	}

	return false
}

func (c *channel) hasCapacity() bool {
	return len(c.c) < c.size
}
