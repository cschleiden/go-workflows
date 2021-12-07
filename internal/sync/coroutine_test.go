package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Coroutine_CanAccessState(t *testing.T) {
	ctx := context.Background()
	c := newState()
	c.Run(ctx, func(ctx context.Context) {
		s := getCoState(ctx)
		require.NotNil(t, s)
	})

	<-c.blocking
}

func Test_Coroutine_MarkedAsDone(t *testing.T) {
	ctx := context.Background()
	c := newState()
	c.Run(ctx, func(ctx context.Context) {

	})

	<-c.blocking

	require.True(t, c.Finished())
}

func Test_Coroutine_MarkedAsBlocked(t *testing.T) {
	ctx := context.Background()
	c := newState()
	c.Run(ctx, func(ctx context.Context) {
		s := getCoState(ctx)

		s.Yield()

		require.FailNow(t, "should not reach this")
	})

	<-c.blocking

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.Finished())
}

func Test_Coroutine_Continue(t *testing.T) {
	ctx := context.Background()
	c := newState()
	c.Run(ctx, func(ctx context.Context) {
		s := getCoState(ctx)

		s.Yield()
	})

	<-c.blocking

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.Finished())

	c.Continue()

	require.False(t, c.blocked.Load().(bool))
	require.True(t, c.Finished())
}

func Test_Coroutine_ContinueAndBlock(t *testing.T) {
	reached := false

	ctx := context.Background()
	c := newState()
	c.Run(ctx, func(ctx context.Context) {
		s := getCoState(ctx)

		s.Yield()

		reached = true

		s.Yield()

		require.FailNow(t, "should not reach this")
	})

	<-c.blocking

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.Finished())

	c.Continue()

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.Finished())
	require.True(t, reached)
}
