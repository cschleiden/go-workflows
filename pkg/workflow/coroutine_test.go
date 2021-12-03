package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Coroutine_CanAccessState(t *testing.T) {
	ctx := context.Background()
	c := newCoroutine(ctx, func(ctx context.Context) {
		s := getCoState(ctx)
		require.NotNil(t, s)
	})

	<-c.blocking
}

func Test_Coroutine_MarkedAsDone(t *testing.T) {
	ctx := context.Background()
	c := newCoroutine(ctx, func(ctx context.Context) {

	})

	<-c.blocking

	require.True(t, c.finished.Load().(bool))
}

func Test_Coroutine_MarkedAsBlocked(t *testing.T) {
	ctx := context.Background()
	c := newCoroutine(ctx, func(ctx context.Context) {
		s := getCoState(ctx)

		s.yield()

		require.FailNow(t, "should not reach this")
	})

	<-c.blocking

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.finished.Load().(bool))
}

func Test_Coroutine_Continue(t *testing.T) {
	ctx := context.Background()
	c := newCoroutine(ctx, func(ctx context.Context) {
		s := getCoState(ctx)

		s.yield()
	})

	<-c.blocking

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.finished.Load().(bool))

	c.cont()

	require.False(t, c.blocked.Load().(bool))
	require.True(t, c.finished.Load().(bool))
}

func Test_Coroutine_ContinueAndBlock(t *testing.T) {
	reached := false

	ctx := context.Background()
	c := newCoroutine(ctx, func(ctx context.Context) {
		s := getCoState(ctx)

		s.yield()

		reached = true

		s.yield()

		require.FailNow(t, "should not reach this")
	})

	<-c.blocking

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.finished.Load().(bool))

	c.cont()

	require.True(t, c.blocked.Load().(bool))
	require.False(t, c.finished.Load().(bool))
	require.True(t, reached)
}
