package tester

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
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

// Test_withRetries_contextCanceled tests that WithRetries returns Canceled
// when the context is already canceled before the function is called
func Test_withRetries_contextCanceled(t *testing.T) {
	wf := func(ctx workflow.Context) error {
		tctx, cancel := workflow.WithCancel(ctx)
		cancel()

		f := workflow.WithRetries(tctx, workflow.DefaultRetryOptions, func(ctx workflow.Context, attempt int) workflow.Future[int] {
			// This should never be called since the context is already canceled
			panic("function should not be called when context is already canceled")
		})

		_, err := f.Get(ctx)
		return err
	}

	tester := NewWorkflowTester[any](wf)

	tester.Execute(context.Background())
	require.True(t, tester.WorkflowFinished())

	_, err := tester.WorkflowResult()
	require.EqualError(t, err, "context canceled")
	tester.AssertExpectations(t)
}

// Test_withRetries_contextCanceledDuringRetry tests that WithRetries returns Canceled
// when the context is canceled during retry backoff
func Test_withRetries_contextCanceledDuringRetry(t *testing.T) {
	wf := func(ctx workflow.Context) (int, error) {
		tctx, cancel := workflow.WithCancel(ctx)

		attemptCount := 0
		f := workflow.WithRetries(tctx, workflow.RetryOptions{
			MaxAttempts:        5,
			FirstRetryInterval: 1 * time.Second,
			BackoffCoefficient: 1,
		}, func(ctx workflow.Context, attempt int) workflow.Future[int] {
			f := sync.NewFuture[int]()
			attemptCount++

			// Fail the first two attempts, then cancel
			if attempt == 1 {
				// Cancel after first retry
				cancel()
			}
			f.Set(attempt, errors.New("retry error"))

			return f
		})

		// Wait for retries to finish
		attempts, err := f.Get(ctx)

		return attempts, err
	}

	tester := NewWorkflowTester[int](wf)

	tester.Execute(context.Background())
	require.True(t, tester.WorkflowFinished())

	_, err := tester.WorkflowResult()
	require.EqualError(t, err, "context canceled")
	tester.AssertExpectations(t)
}

// Test_withRetries_contextCanceledDuringExecution tests that WithRetries returns Canceled
// when the context is canceled while the function is executing
func Test_withRetries_contextCanceledDuringExecution(t *testing.T) {
	wf := func(ctx workflow.Context) error {
		tctx, cancel := workflow.WithCancel(ctx)

		f := workflow.WithRetries(tctx, workflow.RetryOptions{
			MaxAttempts:        3,
			FirstRetryInterval: 1 * time.Second,
			BackoffCoefficient: 1,
		}, func(ctx workflow.Context, attempt int) workflow.Future[int] {
			f := sync.NewFuture[int]()

			// Cancel after first attempt starts
			if attempt == 0 {
				cancel()
			}

			// Simulate async work with an error
			f.Set(attempt, errors.New("operation failed"))

			return f
		})

		_, err := f.Get(ctx)
		return err
	}

	tester := NewWorkflowTester[any](wf)

	tester.Execute(context.Background())
	require.True(t, tester.WorkflowFinished())

	_, err := tester.WorkflowResult()
	require.EqualError(t, err, "context canceled")
	tester.AssertExpectations(t)
}

// Test_withRetries_timerCancellation tests that the retry backoff timer is canceled
// when the context is canceled during the wait
func Test_withRetries_timerCancellation(t *testing.T) {
	wf := func(ctx workflow.Context) error {
		tctx, cancel := workflow.WithCancel(ctx)

		f := workflow.WithRetries(tctx, workflow.RetryOptions{
			MaxAttempts:        5,
			FirstRetryInterval: 10 * time.Second,
			BackoffCoefficient: 1,
		}, func(ctx workflow.Context, attempt int) workflow.Future[int] {
			f := sync.NewFuture[int]()

			// Always fail to trigger retries
			f.Set(attempt, errors.New("operation failed"))

			return f
		})

		// Start a timer to cancel the context after a short delay
		workflow.Go(ctx, func(ctx workflow.Context) {
			workflow.Sleep(ctx, 2*time.Second)
			cancel()
		})

		_, err := f.Get(ctx)
		return err
	}

	tester := NewWorkflowTester[any](wf)

	tester.Execute(context.Background())
	require.True(t, tester.WorkflowFinished())

	_, err := tester.WorkflowResult()
	require.EqualError(t, err, "context canceled")
	tester.AssertExpectations(t)
}

