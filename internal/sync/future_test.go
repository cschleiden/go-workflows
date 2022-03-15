package sync

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_FutureYields(t *testing.T) {
	f := NewFuture[int]()

	c := NewCoroutine(Background(), func(ctx Context) error {
		f.Get(ctx)

		return nil
	})

	c.Execute()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())
}

func Test_FutureSetPanicsWhenSetTwice(t *testing.T) {
	f := NewFuture[int]()

	f.Set(42, nil)

	require.Panics(t, func() {
		f.Set(42, nil)
	})
}

func Test_FutureSetUnblocks(t *testing.T) {
	f := NewFuture[int]()

	var v int

	c := NewCoroutine(Background(), func(ctx Context) error {
		v, _ = f.Get(ctx)

		return nil
	})

	c.Execute()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())

	c.Execute()

	require.False(t, c.Progress())

	f.Set(42, nil)

	c.Execute()

	require.True(t, c.Finished())
	require.False(t, c.Blocked())

	require.True(t, c.Progress())

	require.Equal(t, 42, v)
}

func Test_FutureGetNil(t *testing.T) {
	ctx := Background()
	f := NewFuture[int]()

	c := NewCoroutine(ctx, func(ctx Context) error {
		f.Get(ctx)

		return nil
	})

	f.Set(0, nil)

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}

func Test_FutureSetNil(t *testing.T) {
	ctx := Background()
	f := NewFuture[int]()

	c := NewCoroutine(ctx, func(ctx Context) error {
		f.Get(ctx)

		return nil
	})

	f.Set(0, nil)

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}

func Test_FutureGetError(t *testing.T) {
	ctx := Background()
	f := NewFuture[int]()

	var err error

	c := NewCoroutine(ctx, func(ctx Context) error {
		_, err = f.Get(ctx)

		return nil
	})

	f.Set(0, errors.New("test"))

	c.Execute()
	require.Nil(t, c.Error())
	require.True(t, c.Finished())

	require.Equal(t, errors.New("test"), err)
}
