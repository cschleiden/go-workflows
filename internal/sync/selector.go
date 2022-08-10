package sync

type SelectCase interface {
	Ready() bool
	Handle(Context)
}

func Await[T any](f Future[T], handler func(ctx Context, f Future[T])) SelectCase {
	return &futureCase[T]{
		f:  f.(*future[T]),
		fn: handler,
	}
}

func Send[T any](c Channel[T], v *T, handler func(ctx Context)) SelectCase {
	return &channelSendCase[T]{
		c:  c.(*channel[T]),
		v:  v,
		fn: handler,
	}
}

func Receive[T any](c Channel[T], handler func(ctx Context, v T, ok bool)) SelectCase {
	return &channelReceiveCase[T]{
		c:  c.(*channel[T]),
		fn: handler,
	}
}

func Default(handler func(ctx Context)) SelectCase {
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

type futureCase[T any] struct {
	f  *future[T]
	fn func(Context, Future[T])
}

func (fc *futureCase[T]) Ready() bool {
	return fc.f.Ready()
}

func (fc *futureCase[T]) Handle(ctx Context) {
	fc.fn(ctx, fc.f)
}

type channelReceiveCase[T any] struct {
	c  *channel[T]
	fn func(Context, T, bool)
}

func (crc *channelReceiveCase[T]) Ready() bool {
	return crc.c.canReceive()
}

func (crc *channelReceiveCase[T]) Handle(ctx Context) {
	v, ok := crc.c.Receive(ctx)
	crc.fn(ctx, v, ok)
}

type channelSendCase[T any] struct {
	c  *channel[T]
	v  *T
	fn func(Context)
}

func (csc *channelSendCase[T]) Ready() bool {
	return csc.c.canSend()
}

func (csc *channelSendCase[T]) Handle(ctx Context) {
	csc.c.Send(ctx, *csc.v)
	csc.fn(ctx)
}

type defaultCase struct {
	fn func(Context)
}

func (dc *defaultCase) Ready() bool {
	return true
}

func (dc *defaultCase) Handle(ctx Context) {
	dc.fn(ctx)
}
