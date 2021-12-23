package sync

import "context"

type Selector interface {
	AddFuture(f Future, handler func(ctx context.Context, f Future))

	AddChannelReceive(c Channel, handler func(ctx context.Context, c Channel))

	AddDefault(handler func())

	Select(ctx context.Context)
}

func NewSelector() Selector {
	return &selector{
		cases: make([]selectorCase, 0),
	}
}

type selector struct {
	cases []selectorCase

	defaultFunc func()
}

func (s *selector) AddFuture(f Future, handler func(ctx context.Context, f Future)) {
	s.cases = append(s.cases, &futureCase{
		f:  f.(*futureImpl),
		fn: handler,
	})
}

func (s *selector) AddChannelReceive(c Channel, handler func(ctx context.Context, c Channel)) {
	s.cases = append(s.cases, &channelCase{
		c:  c.(*channel),
		fn: handler,
	})
}

func (s *selector) AddDefault(handler func()) {
	s.defaultFunc = handler
}

func (s *selector) Select(ctx context.Context) {
	cs := getCoState(ctx)

	for {
		// Is any case ready?
		for i, c := range s.cases {
			if c.Ready() {
				c.Handle(ctx)

				// Remove handled case
				s.cases = append(s.cases[:i], s.cases[i+1:]...)
				return
			}
		}

		if s.defaultFunc != nil {
			s.defaultFunc()
			return
		}

		// else, yield and wait for result
		cs.Yield()
	}
}

type selectorCase interface {
	Ready() bool
	Handle(ctx context.Context)
}

var _ = selectorCase(&futureCase{})

type futureCase struct {
	f  *futureImpl
	fn func(context.Context, Future)
}

func (fc *futureCase) Ready() bool {
	return fc.f.Ready()
}

func (fc *futureCase) Handle(ctx context.Context) {
	fc.fn(ctx, fc.f)
}

type channelCase struct {
	c  *channel
	fn func(context.Context, Channel)
}

func (cc *channelCase) Ready() bool {
	return false // TODO
}

func (cc *channelCase) Handle(ctx context.Context) {
	cc.fn(ctx, cc.c)
}
