package sync

import (
	"context"
	"fmt"
	"log"
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

func Test_OneCoroutineAtATime(t *testing.T) {
	s := NewScheduler()

	active := false

	ctx := context.Background()
	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 5; i++ {
			log.Println("enter 1")
			require.False(t, active)

			active = true
			time.Sleep(time.Millisecond * 10)
			active = false

			log.Println("exit 1")
			getCoState(ctx).Yield()
		}
	})

	s.NewCoroutine(ctx, func(ctx context.Context) {
		for i := 0; i < 5; i++ {
			log.Println("enter 2")
			require.False(t, active)

			active = true
			time.Sleep(time.Millisecond * 10)
			active = false

			log.Println("exit 2")
			getCoState(ctx).Yield()
		}
	})

	for i := 0; i < 10; i++ {
		s.Execute(ctx)
	}

	require.Equal(t, 0, s.RunningCoroutines())
}
