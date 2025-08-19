package workflow

import "github.com/cschleiden/go-workflows/internal/sync"

type SelectCase = sync.SelectCase

// Select is the workflow-save equivalent of the select statement.
func Select(ctx Context, cases ...SelectCase) {
	sync.Select(ctx, cases...)
}

// Await calls the provided handler when the given future is ready.
func Await[T any](f Future[T], handler func(Context, Future[T])) SelectCase {
	return sync.Await(f, func(ctx sync.Context, f sync.Future[T]) {
		handler(ctx, f)
	})
}

// Receive calls the provided handler if the given channel can receive a value. The handler receives
// the received value, and the ok flag indicating whether the value was received or the channel was closed.
func Receive[T any](c Channel[T], handler func(ctx Context, v T, ok bool)) SelectCase {
	return sync.Receive(c, handler)
}

// Send calls the provided handler if the given value can be sent to the channel.
func Send[T any](c Channel[T], value *T, handler func(ctx Context)) SelectCase {
	return sync.Send(c, value, handler)
}

// Default calls the provided handler if none of the other cases match.
func Default(handler func(Context)) SelectCase {
	return sync.Default(handler)
}
