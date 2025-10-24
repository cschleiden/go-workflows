package workflow

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestWithRetries(t *testing.T) {
	tests := []struct {
		name           string
		setupCtx       func(Context) (Context, func()) // returns context and cleanup function
		retryOptions   RetryOptions
		fn             func(t *testing.T, attemptCount *int) func(Context, int) Future[string]
		expectedErr    error
		expectedResult string
		expectedCalls  int
	}{
		{
			name: "context canceled before execution",
			setupCtx: func(ctx Context) (Context, func()) {
				ctx, cancel := WithCancel(ctx)
				cancel() // Cancel immediately
				return ctx, func() {}
			},
			retryOptions: DefaultRetryOptions,
			fn: func(t *testing.T, attemptCount *int) func(Context, int) Future[string] {
				return func(ctx Context, attempt int) Future[string] {
					require.FailNow(t, "function should not be called when context is already canceled")
					return sync.NewFuture[string]()
				}
			},
			expectedErr:   Canceled,
			expectedCalls: 0,
		},
		{
			name: "no retry needed - success",
			setupCtx: func(ctx Context) (Context, func()) {
				return ctx, func() {}
			},
			retryOptions: DefaultRetryOptions,
			fn: func(t *testing.T, attemptCount *int) func(Context, int) Future[string] {
				return func(ctx Context, attempt int) Future[string] {
					f := sync.NewFuture[string]()
					*attemptCount++
					f.Set("success", nil)
					return f
				}
			},
			expectedResult: "success",
			expectedCalls:  1,
		},
		{
			name: "max attempts one - no retries",
			setupCtx: func(ctx Context) (Context, func()) {
				return ctx, func() {}
			},
			retryOptions: RetryOptions{
				MaxAttempts:        1,
				FirstRetryInterval: 1 * time.Second,
				BackoffCoefficient: 1,
			},
			fn: func(t *testing.T, attemptCount *int) func(Context, int) Future[string] {
				return func(ctx Context, attempt int) Future[string] {
					f := sync.NewFuture[string]()
					*attemptCount++
					f.Set("", errors.New("error"))
					return f
				}
			},
			expectedErr:   errors.New("error"), // Only compare error message in assertion
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := workflowstate.NewWorkflowState(
				core.NewWorkflowInstance("a", ""), slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New())

			ctx := sync.Background()
			ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)
			ctx = workflowstate.WithWorkflowState(ctx, state)

			ctx, cleanup := tt.setupCtx(ctx)
			defer cleanup()

			attemptCount := 0
			c := sync.NewCoroutine(ctx, func(ctx Context) error {
				f := WithRetries(ctx, tt.retryOptions, tt.fn(t, &attemptCount))

				result, err := f.Get(ctx)

				if tt.expectedErr != nil {
					if tt.expectedErr == Canceled {
						require.Equal(t, Canceled, err)
					} else {
						require.EqualError(t, err, tt.expectedErr.Error())
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, tt.expectedResult, result)
				}

				return nil
			})

			c.Execute()
			require.True(t, c.Finished())
			require.Equal(t, tt.expectedCalls, attemptCount)
		})
	}
}

// Test_WithRetries_PermanentError is kept separate as it has different async behavior
func Test_WithRetries_PermanentError(t *testing.T) {
	state := workflowstate.NewWorkflowState(
		core.NewWorkflowInstance("a", ""), slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New())

	ctx := sync.Background()
	ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(ctx, state)

	attemptCount := 0
	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		_ = WithRetries(ctx, DefaultRetryOptions, func(ctx Context, attempt int) Future[int] {
			f := sync.NewFuture[int]()
			attemptCount++

			if attempt > 0 {
				// Return permanent error on retry
				f.Set(attempt, workflowerrors.NewPermanentError(errors.New("permanent error")))
			} else {
				// Return regular error on first attempt
				f.Set(attempt, errors.New("generic error"))
			}

			return f
		})

		// The future won't be ready immediately since WithRetries spawns a goroutine
		return nil
	})

	c.Execute()

	// Workflow finishes immediately as it just calls WithRetries, which starts an async goroutine
	require.True(t, c.Finished())
	require.Equal(t, 1, attemptCount) // Only first attempt happens synchronously
}
