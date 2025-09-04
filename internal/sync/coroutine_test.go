package sync

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Coroutine_CanAccessState(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)
		require.NotNil(t, s)

		return nil
	})

	c.Execute()
}

func Test_Coroutine_MarkedAsDone(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		return nil
	})

	c.Execute()

	require.True(t, c.Finished())
}

func Test_Coroutine_MarkedAsBlocked(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)

		s.Yield()

		require.FailNow(t, "should not reach this")

		return nil
	})

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())
}

func Test_Coroutine_Continue(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)
		s.Yield()

		return nil
	})

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())

	c.Execute()

	require.False(t, c.Blocked())
	require.True(t, c.Finished())
}

func Test_Coroutine_Continue_WhenFinished(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		return nil
	})

	c.Execute()

	require.True(t, c.Finished())

	c.Execute()

	require.True(t, c.Finished())
}

func Test_Coroutine_ContinueAndBlock(t *testing.T) {
	reached := false

	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)

		s.Yield()

		reached = true

		s.Yield()

		require.FailNow(t, "should not reach this")

		return nil
	})

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())

	c.Execute()

	require.True(t, c.Blocked())
	require.False(t, c.Finished())
	require.True(t, reached)
}

func Test_Coroutine_Exit_Before_Yield(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)

		s.Yield()

		require.FailNow(t, "should not reach this")

		return nil
	})

	r := runtime.NumGoroutine()
	c.Exit()

	require.True(t, c.Finished())
	require.Equal(t, r-1, runtime.NumGoroutine())
}

func Test_Coroutine_Exit(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)

		s.Yield()

		require.FailNow(t, "should not reach this")

		return nil
	})

	c.Execute()

	r := runtime.NumGoroutine()
	c.Exit()

	require.True(t, c.Finished())
	require.Equal(t, r-1, runtime.NumGoroutine())
}

func Test_Coroutine_Exit_with_defer(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)

		defer func() {
			s.Yield()
		}()

		s.Yield()

		require.FailNow(t, "should not reach this")

		return nil
	})

	c.Execute()

	r := runtime.NumGoroutine()
	c.Exit()

	require.True(t, c.Finished())
	require.NoError(t, c.Error())
	require.Equal(t, r-1, runtime.NumGoroutine())
}

func Test_Coroutine_ExitIfAlreadyFinished(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		// Complete immediately
		return nil
	})

	c.Exit()

	require.True(t, c.Finished())
}

func Test_Coroutine_PanicsWhenDeadlocked(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		s := getCoState(ctx)
		s.deadlockDetection = time.Millisecond
		s.Yield()

		time.Sleep(10 * time.Second)

		return nil
	})

	c.Execute()

	require.Panics(t, func() {
		c.Execute()
	})
}

func Test_Coroutine_Error(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		return errors.New("custom error")
	})

	c.Execute()

	require.True(t, c.Finished())
	require.Error(t, c.Error())
	require.Equal(t, "custom error", c.Error().Error())
}

func Test_Coroutine_Panic(t *testing.T) {
	c := NewCoroutine(Background(), func(ctx Context) error {
		panic("test panic")
	})

	c.Execute()

	require.True(t, c.Finished())
	require.Error(t, c.Error())
	require.Equal(t, "panic: test panic", c.Error().Error())
}
