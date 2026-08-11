package tester

import (
	"context"
	"fmt"
	"testing"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Reproduces https://github.com/cschleiden/go-workflows/issues/377
func benchWF(ctx workflow.Context, n int) error {
	wg := workflow.NewWaitGroup()
	for range n {
		wg.Add(1)
		workflow.Go(ctx, func(ctx workflow.Context) {
			defer wg.Done()
			workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, benchAct).Get(ctx)
		})
	}
	wg.Wait(ctx)
	return nil
}

func benchAct(ctx context.Context) error {
	return nil
}

func BenchmarkWorkflowGo(b *testing.B) {
	run := func(b *testing.B, n int) {
		b.Run("n="+fmt.Sprint(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				wft := NewWorkflowTester[any](benchWF)
				wft.OnActivity(benchAct, mock.Anything).Return(nil)
				wft.Execute(context.Background(), n)
				require.True(b, wft.WorkflowFinished())
				_, err := wft.WorkflowResult()
				require.NoError(b, err)
			}
		})
	}
	run(b, 1)
	run(b, 32)
	run(b, 64)
	run(b, 128)
	run(b, 256)
}
