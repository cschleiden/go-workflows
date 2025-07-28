package tester

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
)

func Test_withRetries_permanent(t *testing.T) {
	wf := func(ctx workflow.Context) (int, error) {
		f := workflow.WithRetries(ctx, workflow.DefaultRetryOptions, func(ctx workflow.Context, attempt int) workflow.Future[int] {
			f := sync.NewFuture[int]()

			if attempt > 0 {
				f.Set(attempt, workflowerrors.NewPermanentError(errors.New("permanent error")))
			} else {
				f.Set(attempt, errors.New("generic error"))
			}

			return f
		})

		// Wait for retries to finish
		attempts, err := f.Get(ctx)

		return attempts, err
	}

	tester := NewWorkflowTester[int](wf)

	tester.Execute(context.Background())
	require.True(t, tester.WorkflowFinished())

	attempts, err := tester.WorkflowResult()
	require.Equal(t, 1, attempts)
	var permanentErr *workflowerrors.Error
	require.ErrorAs(t, err, &permanentErr)
	require.True(t, permanentErr.Permanent)
	tester.AssertExpectations(t)

	require.Equal(t, 1, attempts)
}
