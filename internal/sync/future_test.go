package sync

import (
	"errors"
	"testing"

	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/stretchr/testify/require"
)

func Test_FutureYields(t *testing.T) {
	f := NewFuture()

	c := NewCoroutine(Background(), func(ctx Context) error {
		var v int
		f.Get(ctx, &v)

		return nil
	})

	c.Execute()

	require.False(t, c.Finished())
	require.True(t, c.Blocked())
}

func Test_FutureSetPanicsWhenSetTwice(t *testing.T) {
	f := NewFuture()

	f.Set(42, nil)

	require.Panics(t, func() {
		f.Set(42, nil)
	})
}

func Test_FutureSetUnblocks(t *testing.T) {
	f := NewFuture()

	var v int

	c := NewCoroutine(Background(), func(ctx Context) error {
		f.Get(ctx, &v)

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
	f := NewFuture()

	c := NewCoroutine(ctx, func(ctx Context) error {
		f.Get(ctx, nil)

		return nil
	})

	f.Set(nil, nil)

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}

func Test_FutureSetNil(t *testing.T) {
	ctx := Background()
	f := NewFuture()

	var r int
	c := NewCoroutine(ctx, func(ctx Context) error {
		f.Get(ctx, &r)

		return nil
	})

	var v payload.Payload
	f.Set(v, nil)

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}

func Test_FutureGetError(t *testing.T) {
	ctx := Background()
	f := NewFuture()

	var err error

	c := NewCoroutine(ctx, func(ctx Context) error {
		err = f.Get(ctx, nil)

		return nil
	})

	f.Set(nil, errors.New("test"))

	c.Execute()
	require.Nil(t, c.Error())
	require.True(t, c.Finished())

	require.Equal(t, errors.New("test"), err)
}
