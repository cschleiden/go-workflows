package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Scheduler(t *testing.T) {
	s := NewScheduler()

	hit := 0

	ctx := Background()
	s.NewCoroutine(ctx, func(ctx Context) error {
		hit++

		getCoState(ctx).Yield()

		return nil
	})

	require.Equal(t, 0, hit)

	s.Execute()
	require.Equal(t, 1, hit)
	require.Equal(t, 1, s.RunningCoroutines())

	// Coroutine is finished
	s.Execute()
	require.Equal(t, 1, hit)
	require.Equal(t, 0, s.RunningCoroutines())
}

func Test_Scheduler_OneCoroutineAtATime(t *testing.T) {
	s := NewScheduler()

	active := false

	ctx := Background()
	s.NewCoroutine(ctx, func(ctx Context) error {
		for i := 0; i < 5; i++ {
			require.False(t, active)
			active = true
			time.Sleep(time.Millisecond * 1)
			active = false
			getCoState(ctx).Yield()
		}

		return nil
	})

	s.NewCoroutine(ctx, func(ctx Context) error {
		for i := 0; i < 5; i++ {
			require.False(t, active)

			active = true
			time.Sleep(time.Millisecond * 1)
			active = false

			getCoState(ctx).Yield()
		}

		return nil
	})

	for i := 0; i < 10; i++ {
		s.Execute()
	}

	require.Equal(t, 0, s.RunningCoroutines())
}

func Test_Scheduler_ExecuteUntilBlocked(t *testing.T) {
	s := NewScheduler()

	hits := 0

	ctx := Background()
	s.NewCoroutine(ctx, func(ctx Context) error {
		for i := 0; i < 4; i++ {
			hits++

			getCoState(ctx).MadeProgress()
			getCoState(ctx).Yield()
		}

		getCoState(ctx).Yield()

		require.Fail(t, "should not be reached")

		return nil
	})

	s.Execute()

	require.Equal(t, 4, hits)
}

func Test_Scheduler_ExecuteUntilAllBlocked(t *testing.T) {
	s := NewScheduler()

	hits := 0

	ctx := Background()
	s.NewCoroutine(ctx, func(ctx Context) error {
		for i := 0; i < 2; i++ {
			hits++

			getCoState(ctx).MadeProgress()
			getCoState(ctx).Yield()
		}

		return nil
	})

	s.NewCoroutine(ctx, func(ctx Context) error {
		for i := 0; i < 4; i++ {
			hits++

			getCoState(ctx).MadeProgress()
			getCoState(ctx).Yield()
		}

		getCoState(ctx).Yield()
		require.Fail(t, "should not be reached")

		return nil
	})

	s.Execute()
	require.Equal(t, 1, s.RunningCoroutines())
	require.Equal(t, 6, hits)
}

func Test_Scheduler_Exit(t *testing.T) {
	s := NewScheduler()

	hits := 0

	ctx := Background()
	s.NewCoroutine(ctx, func(ctx Context) error {
		for {
			hits++
			getCoState(ctx).Yield()
		}
	})

	s.Execute()

	s.Exit()

	s.Execute()
	s.Execute()

	require.Equal(t, 1, hits)
}

func Test_Scheduler_Panic(t *testing.T) {
	s := NewScheduler()

	ctx := Background()
	s.NewCoroutine(ctx, func(ctx Context) error {
		panic("something went wrong")
	})

	err := s.Execute()

	require.Error(t, err)
	require.Equal(t, "panic: something went wrong", err.Error())
	require.Equal(t, 0, s.RunningCoroutines())
}
