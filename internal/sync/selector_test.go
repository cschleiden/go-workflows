package sync

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_FutureSelector_SelectWaits(t *testing.T) {
	s := NewSelector()

	ctx := context.Background()
	cs := newState()
	ctx = withCoState(ctx, cs)

	f := NewFuture(cs)

	fn := func() {
		f.Set(func(v interface{}) error {
			x := reflect.ValueOf(v)
			x.Elem().Set(reflect.ValueOf(42))

			return nil
		})
	}

	s.AddFuture(f, func(f Future) {
		var r int
		err := f.Get(&r)
		require.Nil(t, err)

		require.Equal(t, 42, r)
	})

	// Wait for result
	s.Select(ctx)

	fn()
}
