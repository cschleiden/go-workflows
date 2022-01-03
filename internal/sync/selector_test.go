package sync

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_FutureSelector_SelectWaits(t *testing.T) {
	ctx := Background()
	f := NewFuture()
	reachedEnd := false

	cr := NewCoroutine(ctx, func(ctx Context) {
		s := NewSelector()

		s.AddFuture(f, func(ctx Context, f Future) {
			var r int
			err := f.Get(ctx, &r)
			require.Nil(t, err)

			require.Equal(t, 42, r)
		})

		// Wait for result
		s.Select(ctx)

		reachedEnd = true
	})

	cr.Execute()
	require.False(t, reachedEnd)

	f.Set(ctx, func(v interface{}) error {
		x := reflect.ValueOf(v)
		x.Elem().Set(reflect.ValueOf(42))
		return nil
	})

	cr.Execute()
	require.True(t, reachedEnd)
}

func Test_FutureSelector_SelectWaitsWithSameOrder(t *testing.T) {
	ctx := Background()

	f := NewFuture()
	f2 := NewFuture()

	reachedEnd := false
	order := make([]int, 0)

	cs := NewCoroutine(ctx, func(ctx Context) {
		s := NewSelector()

		s.AddFuture(f, func(ctx Context, f Future) {
			var r int
			err := f.Get(ctx, &r)
			require.Nil(t, err)
			require.Equal(t, 42, r)
			order = append(order, 42)
		})

		s.AddFuture(f2, func(ctx Context, f Future) {
			var r int
			err := f.Get(ctx, &r)
			require.Nil(t, err)
			require.Equal(t, 23, r)
			order = append(order, 23)
		})

		// Wait for result
		s.Select(ctx)
		s.Select(ctx)

		reachedEnd = true
	})

	cs.Execute()

	require.False(t, reachedEnd)

	f.Set(ctx, func(v interface{}) error {
		x := reflect.ValueOf(v)
		x.Elem().Set(reflect.ValueOf(42))

		return nil
	})

	f2.Set(ctx, func(v interface{}) error {
		x := reflect.ValueOf(v)
		x.Elem().Set(reflect.ValueOf(23))

		return nil
	})

	cs.Execute()

	require.True(t, cs.Finished())
	require.True(t, reachedEnd)
	require.Equal(t, []int{42, 23}, order)
}

func Test_FutureSelector_DefaultCase(t *testing.T) {
	f := NewFuture()

	defaultHandled := false
	reachedEnd := false

	cs := NewCoroutine(Background(), func(ctx Context) {
		s := NewSelector()

		s.AddFuture(f, func(_ Context, _ Future) {
			require.Fail(t, "should not be called")
		})

		s.AddDefault(func() {
			defaultHandled = true
		})

		// Wait for result
		s.Select(ctx)

		reachedEnd = true
	})

	cs.Execute()

	require.True(t, reachedEnd)
	require.True(t, defaultHandled)
}

func Test_ChannelSelector_Select(t *testing.T) {
	c := NewChannel()

	reachedEnd := false

	ctx := Background()

	var r int

	cr := NewCoroutine(ctx, func(ctx Context) {
		s := NewSelector()

		s.AddChannelReceive(c, func(ctx Context, c Channel) {
			c.Receive(ctx, &r)
		})

		// Wait for result
		s.Select(ctx)

		reachedEnd = true
	})

	cr.Execute()

	NewCoroutine(ctx, func(ctx Context) {
		c.Send(ctx, 42)
	}).Execute()

	cr.Execute()

	require.True(t, reachedEnd)
	require.Equal(t, 42, r)
}
