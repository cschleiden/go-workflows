package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type ctxKey int

func TestWithValue(t *testing.T) {
	ctx := WithValue(Background(), ctxKey(42), "foo")
	require.Equal(t, "foo", ctx.Value(ctxKey(42)))
}

func TestWithCancel(t *testing.T) {
	c2, cancel := WithCancel(Background())
	require.NotNil(t, c2.Done())

	canceled := false

	cr := NewCoroutine(c2, func(ctx Context) {
		// Create child context, canceled when parent is canceled
		ctx, _ = WithCancel(ctx)

		s := NewSelector()

		s.AddChannelReceive(ctx.Done(), func(ctx Context, c Channel) {
			canceled = true
		})

		s.Select(ctx)
	})

	cr.Execute()
	require.False(t, cr.Finished())

	cancel()

	cr.Execute()
	require.True(t, cr.Finished())

	require.True(t, canceled)
}

// TODO: Support custom context implementations with their own `Done` channel
// type myDoneCtx struct {
// 	Context
// }

// func (d *myDoneCtx) Done() Channel {
// 	c := NewChannel()
// 	return c
// }

// func TestWithOtherCancel(t *testing.T) {
// 	canceled := false

// 	var cancel CancelFunc

// 	cr := NewCoroutine(Background(), func(ctx Context) {
// 		var c2 Context
// 		c2, cancel = WithCancel(&myDoneCtx{ctx})

// 		s := NewSelector()
// 		s.AddChannelReceive(c2.Done(), func(ctx Context, c Channel) {
// 			canceled = true
// 		})
// 		s.Select(ctx)
// 	})

// 	cr.Execute()
// 	require.False(t, cr.Finished())

// 	cancel()

// 	cr.Execute()
// 	require.True(t, cr.Finished())

// 	require.True(t, canceled)
// }
