package workflow

import (
	"math"
	"time"

	"github.com/cschleiden/go-workflows/internal/sync"
)

type RetryOptions struct {
	// Maximum number of times to retry
	MaxAttempts int

	// Time to wait before first retry
	FirstRetryInterval time.Duration

	// Maximum delay for any individual retry attempt
	MaxRetryInterval time.Duration

	// Coeffecient for calculation the next retry delay
	BackoffCoefficient float64

	// Timeout after which retries are aborted
	RetryTimeout time.Duration
}

var DefaultRetryOptions = RetryOptions{
	MaxAttempts:        3,
	BackoffCoefficient: 1,
}

func withRetries[T any](ctx sync.Context, retryOptions RetryOptions, fn func(ctx sync.Context, attempt int) Future[T]) Future[T] {
	attempt := 0
	firstAttempt := Now(ctx)

	f := fn(ctx, attempt)

	if retryOptions.MaxAttempts <= 1 {
		// Short-circuit if we don't need to retry
		return f
	}

	// Start a separate co-routine for retries
	r := sync.NewFuture[T]()

	sync.Go(ctx, func(ctx sync.Context) {
		var result T
		var err error

		var retryExpiration time.Time
		if retryOptions.RetryTimeout != 0 {
			retryExpiration = firstAttempt.Add(retryOptions.RetryTimeout)
		}

		for {
			// Wait for active operation to finish
			result, err = f.Get(ctx)
			if err == nil {
				break
			}

			if err == sync.Canceled {
				break
			}

			backoffDuration := time.Duration(float64(retryOptions.FirstRetryInterval) * math.Pow(retryOptions.BackoffCoefficient, float64(attempt)))
			if retryOptions.MaxRetryInterval > 0 {
				backoffDuration = time.Duration(math.Min(float64(backoffDuration), float64(retryOptions.MaxRetryInterval)))
			}

			if err := Sleep(ctx, backoffDuration); err != nil {
				r.Set(*new(T), err)
				return
			}

			if !retryExpiration.IsZero() && Now(ctx).After(retryExpiration) {
				// Reached maximum retry time, abort retries
				break
			}

			attempt++
			if attempt >= retryOptions.MaxAttempts {
				break
			}

			f = fn(ctx, attempt+1)
		}

		r.Set(result, err)
	})

	return r
}
