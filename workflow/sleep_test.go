package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cschleiden/go-workflows/internal/sync"
)

func Test_Sleep_Yields(t *testing.T) {
	ctx := sync.Background()

	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		Sleep(ctx, 2*time.Millisecond)
		require.FailNow(t, "should not reach this")

		return nil
	})

	c.Execute()
}
