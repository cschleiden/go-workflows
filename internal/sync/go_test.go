package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Go(t *testing.T) {
	ctx := Background()
	s := NewScheduler()

	called := false

	s.NewCoroutine(ctx, func(ctx Context) error {
		Go(ctx, func(ctx Context) {
			called = true
		})

		return nil
	})

	err := s.Execute()
	require.NoError(t, err)
	require.True(t, called)
}

func Test_Go_MultipleGoroutines(t *testing.T) {
	ctx := Background()
	s := NewScheduler()

	called := 0

	s.NewCoroutine(ctx, func(ctx Context) error {
		Go(ctx, func(ctx Context) {
			called++
		})

		Go(ctx, func(ctx Context) {
			called++
		})

		return nil
	})

	err := s.Execute()
	require.NoError(t, err)
	require.Equal(t, 2, called)
}
