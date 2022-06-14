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

	cr := NewCoroutine(c2, func(ctx Context) error {
		// Create child context, canceled when parent is canceled
		ctx, _ = WithCancel(ctx)

		Select(
			ctx,
			Receive(ctx.Done(), func(ctx Context, _ struct{}, _ bool) {
				canceled = true
			}),
		)

		return nil
	})

	cr.Execute()
	require.False(t, cr.Finished())

	cancel()

	cr.Execute()
	require.True(t, cr.Finished())

	require.True(t, canceled)
}
