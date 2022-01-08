package sync

import (
	"errors"
	"testing"

	"github.com/cschleiden/go-dt/internal/payload"
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

	f.Set(42, nil)

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

	f.Set(nil, nil)

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}

func Test_SetNil(t *testing.T) {
	ctx := Background()
	f := NewFuture()

	var r int
	c := NewCoroutine(ctx, func(ctx Context) {
		f.Get(ctx, &r)
	})

	var v payload.Payload
	f.Set(v, nil)

	c.Execute()
	require.Nil(t, c.Error())

	require.True(t, c.Finished())
}

func Test_GetError(t *testing.T) {
	ctx := Background()
	f := NewFuture()

	var err error

	c := NewCoroutine(ctx, func(ctx Context) {
		err = f.Get(ctx, nil)
	})

	f.Set(nil, errors.New("test"))

	c.Execute()
	require.Nil(t, c.Error())
	require.True(t, c.Finished())

	require.Equal(t, errors.New("test"), err)
}
