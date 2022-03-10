package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

type Future = sync.Future
type Channel = sync.Channel
type Context = sync.Context
type WaitGroup = sync.WaitGroup

var Canceled = sync.Canceled

func Go(ctx Context, f func(ctx Context)) {
	sync.Go(ctx, f)
}

type SelectCase = sync.SelectCase

func Select(ctx Context, cases ...SelectCase) {
	sync.Select(ctx, cases...)
}

func Await(f Future, handler func(Context, Future)) SelectCase {
	return sync.Await(f, handler)
}

func Receive(c Channel, handler func(Context, Channel)) SelectCase {
	return sync.Receive(c, handler)
}

func Default(handler func(Context)) SelectCase {
	return sync.Default(handler)
}

func NewChannel() Channel {
	return sync.NewChannel()
}

func NewBufferedChannel(size int) Channel {
	return sync.NewBufferedChannel(size)
}

func NewWaitGroup() WaitGroup {
	return sync.NewWaitGroup()
}
