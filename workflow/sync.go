package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

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

func Await[T any](f Future[T], handler func(Context, Future[T])) SelectCase {
	return sync.Await[T](f, func(ctx sync.Context, f sync.Future[T]) {
		handler(ctx, f)
	})
}

func Receive[T any](c Channel[T], handler func(ctx Context, v T, ok bool)) SelectCase {
	return sync.Receive[T](c, func(ctx sync.Context, v T, ok bool) {
		handler(ctx, v, ok)
	})
}

func Default(handler func(Context)) SelectCase {
	return sync.Default(handler)
}

func NewWaitGroup() WaitGroup {
	return sync.NewWaitGroup()
}
