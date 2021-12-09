package sync

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Yields(t *testing.T) {
	c := NewCoroutine()

	f := NewFuture(c)

	c.Run(context.Background(), func(_ context.Context) {
		var v int
		f.Get(&v)
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
		f.Get(&v)
	})
	c.WaitUntilBlocked()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())

	f.Set(func(v interface{}) error {
		r := reflect.ValueOf(v)
		r.Elem().Set(reflect.ValueOf(42))

		return nil
	})

	c.Continue()

	require.True(t, c.Finished())
	require.False(t, c.Blocked())

	require.Equal(t, 42, v)
}
