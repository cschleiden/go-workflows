package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_FutureSelector_SelectWaits(t *testing.T) {
	ctx := Background()
	f := NewFuture[int]()
	reachedEnd := false

	cr := NewCoroutine(ctx, func(ctx Context) error {
		Select(
			ctx,
			Await(f.(Future[int]), func(ctx Context, f Future[int]) {
				r, err := f.Get(ctx)
				require.Nil(t, err)
				require.Equal(t, 42, r)
			}),
		)

		reachedEnd = true

		return nil
	})

	cr.Execute()
	require.False(t, reachedEnd)

	f.Set(42, nil)

	cr.Execute()
	require.True(t, reachedEnd)
}

func Test_FutureSelector_SelectWaitsWithSameOrder(t *testing.T) {
	ctx := Background()

	f := NewFuture[int]()
	f2 := NewFuture[int]()

	reachedEnd := false
	order := make([]int, 0)

	cs := NewCoroutine(ctx, func(ctx Context) error {
		for i := 0; i < 2; i++ {
			Select(
				ctx,
				Await(f.(Future[int]), func(ctx Context, f Future[int]) {
					r, err := f.Get(ctx)
					require.Nil(t, err)
					require.Equal(t, 42, r)
					order = append(order, 42)
				}),
				Await(f2.(Future[int]), func(ctx Context, f Future[int]) {
					r, err := f.Get(ctx)
					require.Nil(t, err)
					require.Equal(t, 23, r)
					order = append(order, 23)
				}),
			)
		}

		reachedEnd = true

		return nil
	})

	cs.Execute()

	require.False(t, reachedEnd)

	f.Set(42, nil)
	f2.Set(23, nil)

	cs.Execute()

	require.True(t, cs.Finished())
	require.True(t, reachedEnd)
	require.Equal(t, []int{42, 42}, order)
}

func Test_FutureSelector_DefaultCase(t *testing.T) {
	f := NewFuture[int]()

	defaultHandled := false
	reachedEnd := false

	cs := NewCoroutine(Background(), func(ctx Context) error {
		Select(
			ctx,
			Await[int](f, func(ctx Context, _ Future[int]) {
				require.Fail(t, "should not be called")
			}),

			Default(func(ctx Context) {
				defaultHandled = true
			}),
		)

		reachedEnd = true

		return nil
	})

	cs.Execute()

	require.True(t, reachedEnd)
	require.True(t, defaultHandled)
}

func Test_ChannelSelector_Select(t *testing.T) {
	c := NewChannel[int]()

	reachedEnd := false

	ctx := Background()

	var r int

	cr := NewCoroutine(ctx, func(ctx Context) error {
		Select(
			ctx,
			Receive(c, func(ctx Context, v int, ok bool) {
				r = v
				require.True(t, ok)
				require.Equal(t, 42, v)
			}),
		)

		reachedEnd = true

		return nil
	})

	cr.Execute()

	NewCoroutine(ctx, func(ctx Context) error {
		c.Send(ctx, 42)

		return nil
	}).Execute()

	cr.Execute()

	require.True(t, reachedEnd)
	require.Equal(t, 42, r)
}
