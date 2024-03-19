package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_WaitGroup_PanicsForInvalidCounters(t *testing.T) {
	wg := NewWaitGroup()

	require.PanicsWithValue(t, "negative WaitGroup counter", func() {
		wg.Add(-2)
	})
}

func Test_WaitGroup_WaitAfterdone(t *testing.T) {
	s := NewScheduler()
	ctx := Background()

	wg := NewWaitGroup()
	wg.Add(2)
	wg.Done()
	wg.Done()

	s.NewCoroutine(ctx, func(ctx Context) error {
		wg.Wait(ctx)

		return nil
	})

	s.Execute()
	require.Equal(t, 0, s.RunningCoroutines())
}

func Test_WaitGroup_Blocks(t *testing.T) {
	s := NewScheduler()
	ctx := Background()

	wg := NewWaitGroup()
	wg.Add(2)

	s.NewCoroutine(ctx, func(ctx Context) error {
		wg.Wait(ctx)

		return nil
	})

	s.Execute()
	require.Equal(t, 1, s.RunningCoroutines())

	s.NewCoroutine(ctx, func(ctx Context) error {
		wg.Done()

		return nil
	})

	s.Execute()
	require.Equal(t, 1, s.RunningCoroutines())

	s.NewCoroutine(ctx, func(ctx Context) error {
		wg.Done()

		return nil
	})

	s.Execute()
	require.Equal(t, 0, s.RunningCoroutines())
}
