package workflow

import "time"

type RetryOptions struct {
	// Maximum number of times to retry
	MaxAttempts int

	FirstRetryInterval time.Duration

	MaxRetryInterval time.Duration

	BackoffCoefficient float64

	RetryTimeout time.Duration
}

var DefaultRetryOptions = RetryOptions{
	MaxAttempts:        3,
	BackoffCoefficient: 1,
}
