package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Yields(t *testing.T) {
	c := NewCoroutine()

	f := NewFuture(c)

	c.Run(context.Background(), func(_ context.Context) {
		f.Get()
	})

	c.WaitUntilBlocked()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())
}

func Test_SetUnblocks(t *testing.T) {
	c := NewCoroutine()

	f := NewFuture(c)

	var v int

	c.Run(context.Background(), func(_ context.Context) {
		i, _ := f.Get()
		v = i.(int)
	})
	c.WaitUntilBlocked()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())

	f.Set(42)

	c.Continue()

	require.True(t, c.Finished())
	require.False(t, c.Blocked())

	require.Equal(t, 42, v)
}
