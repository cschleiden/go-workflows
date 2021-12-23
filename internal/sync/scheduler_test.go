package sync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Scheduler(t *testing.T) {
	s := NewScheduler()

	hit := 0

	ctx := context.Background()
	s.NewCoroutine(ctx, func(ctx context.Context) {
		hit++

		getCoState(ctx).Yield()

		fmt.Println("tes")
	})

	require.Equal(t, 0, hit)

	s.Execute(ctx)
	require.Equal(t, 1, hit)
	require.Equal(t, 1, s.RunningCoroutines())

	// Coroutine is finished
	s.Execute(ctx)
	require.Equal(t, 1, hit)
	require.Equal(t, 0, s.RunningCoroutines())
}

func Test_Scheduler_OneCoroutineAtATime(t *testing.T) {
	s := NewScheduler()

	active := false

	ctx := context.Background()
	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 5; i++ {
			require.False(t, active)
			active = true
			time.Sleep(time.Millisecond * 1)
			active = false
			getCoState(ctx).Yield()
		}
	})

	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 5; i++ {
			require.False(t, active)

			active = true
			time.Sleep(time.Millisecond * 1)
			active = false

			getCoState(ctx).Yield()
		}
	})

	for i := 0; i < 10; i++ {
		s.Execute(ctx)
	}

	require.Equal(t, 0, s.RunningCoroutines())
}

func Test_Scheduler_ExecuteUntilBlocked(t *testing.T) {
	s := NewScheduler()

	hits := 0

	ctx := context.Background()
	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 4; i++ {
			hits++

			getCoState(ctx).MadeProgress()
			getCoState(ctx).Yield()
		}

		getCoState(ctx).Yield()

		require.Fail(t, "should not be reached")
	})

	s.Execute(ctx)

	require.Equal(t, 4, hits)
}

func Test_Scheduler_ExecuteUntilAllBlocked(t *testing.T) {
	s := NewScheduler()

	hits := 0

	ctx := context.Background()
	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 2; i++ {
			hits++

			getCoState(ctx).MadeProgress()
			getCoState(ctx).Yield()
		}
	})

	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 4; i++ {
			hits++

			getCoState(ctx).MadeProgress()
			getCoState(ctx).Yield()
		}

		getCoState(ctx).Yield()
		require.Fail(t, "should not be reached")
	})

	s.Execute(ctx)
	require.Equal(t, 1, s.RunningCoroutines())
	require.Equal(t, 6, hits)
}

func Test_Scheduler_Exit(t *testing.T) {
	s := NewScheduler()

	hits := 0

	ctx := context.Background()
	s.NewCoroutine(ctx, func(ctx context.Context) {
		for {
			hits++
			getCoState(ctx).Yield()
		}
	})

	s.Execute(ctx)

	s.Exit(ctx)

	s.Execute(ctx)
	s.Execute(ctx)

	require.Equal(t, 1, hits)
}
