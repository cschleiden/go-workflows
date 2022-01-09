package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Coroutine_CanAccessState(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
		s := getCoState(ctx)
		require.NotNil(t, s)
	})

	c.Execute()
}

func Test_Coroutine_MarkedAsDone(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
	})

	c.Execute()

	require.True(t, c.Finished())
}

func Test_Coroutine_MarkedAsBlocked(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
		s := getCoState(ctx)

		s.Yield()

		require.FailNow(t, "should not reach this")
	})

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())
}

func Test_Coroutine_Continue(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
		s := getCoState(ctx)
		s.Yield()
	})

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())

	c.Execute()

	require.False(t, c.Blocked())
	require.True(t, c.Finished())
}

func Test_Coroutine_Continue_WhenFinished(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
	})

	c.Execute()

	require.True(t, c.Finished())

	c.Execute()

	require.True(t, c.Finished())
}

func Test_Coroutine_ContinueAndBlock(t *testing.T) {
	reached := false

	c := NewCoroutine(Background(), func(ctx Context) {
		s := getCoState(ctx)

		s.Yield()

		reached = true

		s.Yield()

		require.FailNow(t, "should not reach this")
	})

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())
	require.True(t, reached)
}

func Test_Exit(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
		s := getCoState(ctx)

		s.Yield()

		require.FailNow(t, "should not reach this")
	})

	c.Exit()

	require.True(t, c.Finished())
}

func Test_ExitIfAlreadyFinished(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
		// Complete immedeiately
	})

	c.Exit()

	require.True(t, c.Finished())
}

func Test_Continue_PanicsWhenDeadlocked(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) {
		s := getCoState(ctx)
		s.deadlockDetection = time.Millisecond
		s.Yield()

		time.Sleep(10 * time.Millisecond)
	})

	c.Execute()

	require.Panics(t, func() {
		c.Execute()
	})
}
