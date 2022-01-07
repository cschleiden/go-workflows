package sync

type Selector interface {
	AddFuture(f Future, handler func(ctx Context, f Future)) Selector

	AddChannelReceive(c Channel, handler func(ctx Context, c Channel)) Selector

	AddDefault(handler func()) Selector

	Select(ctx Context)
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

func (s *selector) AddFuture(f Future, handler func(ctx Context, f Future)) Selector {
	s.cases = append(s.cases, &futureCase{
		f:  f.(*futureImpl),
		fn: handler,
	})

	return s
}

func (s *selector) AddChannelReceive(c Channel, handler func(ctx Context, c Channel)) Selector {
	channel := c.(*channel)

	s.cases = append(s.cases, &channelCase{
		c:  channel,
		fn: handler,
	})

	return s
}

func (s *selector) AddDefault(handler func()) Selector {
	s.defaultFunc = handler

	return s
}

func (s *selector) Select(ctx Context) {
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
	Handle(ctx Context)
}

var _ = selectorCase(&futureCase{})

type futureCase struct {
	f  *futureImpl
	fn func(Context, Future)
}

func (fc *futureCase) Ready() bool {
	return fc.f.Ready()
}

func (fc *futureCase) Handle(ctx Context) {
	fc.fn(ctx, fc.f)
}

type channelCase struct {
	c  *channel
	fn func(Context, Channel)
}

func (cc *channelCase) Ready() bool {
	return cc.c.canReceive()
}

func (cc *channelCase) Handle(ctx Context) {
	cc.fn(ctx, cc.c)
}
