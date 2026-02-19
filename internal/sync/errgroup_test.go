package sync

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ErrGroup_Success(t *testing.T) {
	s := NewScheduler()
	ctx := Background()

	s.NewCoroutine(ctx, func(ctx Context) error {
		gctx, g := WithErrGroup(ctx)

		g.Go(func(ctx Context) error { return nil })
		g.Go(func(ctx Context) error { return nil })

		err := g.Wait(gctx)
		require.NoError(t, err)
		return nil
	})

	err := s.Execute()
	require.NoError(t, err)
	require.Equal(t, 0, s.RunningCoroutines())
}

func Test_ErrGroup_FirstError(t *testing.T) {
	s := NewScheduler()
	ctx := Background()

	s.NewCoroutine(ctx, func(ctx Context) error {
		gctx, g := WithErrGroup(ctx)

		e1 := errors.New("boom")

		g.Go(func(ctx Context) error { return e1 })
		g.Go(func(ctx Context) error { return nil })

		err := g.Wait(gctx)
		require.Equal(t, e1, err)
		return nil
	})

	err := s.Execute()
	require.NoError(t, err)
}

func Test_ErrGroup_MultipleErrors_FirstWins(t *testing.T) {
	s := NewScheduler()
	ctx := Background()

	s.NewCoroutine(ctx, func(ctx Context) error {
		gctx, g := WithErrGroup(ctx)

		e1 := errors.New("first")
		e2 := errors.New("second")

		g.Go(func(ctx Context) error { return e1 })
		g.Go(func(ctx Context) error { return e2 })

		err := g.Wait(gctx)
		require.Equal(t, e1, err)
		return nil
	})

	err := s.Execute()
	require.NoError(t, err)
}
