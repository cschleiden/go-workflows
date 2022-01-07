package sync

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Yields(t *testing.T) {
	f := NewFuture()

	c := NewCoroutine(Background(), func(ctx Context) {
		var v int
		f.Get(ctx, &v)
	})

	c.Execute()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())
}

func Test_SetUnblocks(t *testing.T) {
	f := NewFuture()

	var v int

	c := NewCoroutine(Background(), func(ctx Context) {
		f.Get(ctx, &v)
	})

	c.Execute()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())

	c.Execute()

	require.False(t, c.Progress())

	f.Set(Background(), func(v interface{}) error {
		r := reflect.ValueOf(v)
		r.Elem().Set(reflect.ValueOf(42))

		return nil
	})

	c.Execute()

	require.True(t, c.Finished())
	require.False(t, c.Blocked())

	require.True(t, c.Progress())

	require.Equal(t, 42, v)
}

func Test_GetNil(t *testing.T) {
	ctx := Background()
	f := NewFuture()

	c := NewCoroutine(ctx, func(ctx Context) {
		f.Get(ctx, nil)
	})

	f.Set(ctx, func(v interface{}) error {
		require.FailNow(t, "should not be called")
		return nil
	})

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}
