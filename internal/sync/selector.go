package sync

// type Selector interface {
// 	AddFuture(f Future, handler func(ctx Context, f Future)) Selector

// 	AddChannelReceive(c Channel, handler func(ctx Context, c Channel)) Selector

// 	AddDefault(handler func()) Selector

// 	Select(ctx Context)
// }

type SelectCase interface {
	Ready() bool
	Handle(ctx Context)
}

func Await(f Future, handler func(Context, Future)) SelectCase {
	return &futureCase{
		f:  f.(*futureImpl),
		fn: handler,
	}
}

func Receive(c Channel, handler func(Context, Channel)) SelectCase {
	channel := c.(*channel)

	return &channelCase{
		c:  channel,
		fn: handler,
	}
}

func Default(handler func(Context)) SelectCase {
	return &defaultCase{
		fn: handler,
	}
}

func Select(ctx Context, cases ...SelectCase) {
	cs := getCoState(ctx)

	for {
		// Is any case ready?
		for _, c := range cases {
			if c.Ready() {
				c.Handle(ctx)
				return
			}
		}

		// else, yield and wait for result
		cs.Yield()
	}
}

var _ = SelectCase(&futureCase{})

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

var _ = SelectCase(&channelCase{})

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

var _ = SelectCase(&defaultCase{})

type defaultCase struct {
	fn func(Context)
}

func (dc *defaultCase) Ready() bool {
	return true
}

func (dc *defaultCase) Handle(ctx Context) {
	dc.fn(ctx)
}
