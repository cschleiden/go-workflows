package redis

import (
	"time"

	"github.com/cschleiden/go-workflows/backend"
)

type RedisOptions struct {
	*backend.Options

	BlockTimeout time.Duration

	AutoExpiration              time.Duration
	AutoExpirationContinueAsNew time.Duration

	KeyPrefix string
}

type RedisBackendOption func(*RedisOptions)

// WithKeyPrefix sets the prefix for all keys used in the Redis backend.
func WithKeyPrefix(prefix string) RedisBackendOption {
	return func(o *RedisOptions) {
		o.KeyPrefix = prefix
	}
}

// WithBlockTimeout sets the timeout for blocking operations like dequeuing a workflow or activity task
func WithBlockTimeout(timeout time.Duration) RedisBackendOption {
	return func(o *RedisOptions) {
		o.BlockTimeout = timeout
	}
}

// WithAutoExpiration sets the duration after which finished runs will expire from the data store.
// If set to 0 (default), runs will never expire and need to be manually removed.
func WithAutoExpiration(expireFinishedRunsAfter time.Duration) RedisBackendOption {
	return func(o *RedisOptions) {
		o.AutoExpiration = expireFinishedRunsAfter
	}
}

// WithAutoExpirationContinueAsNew sets the duration after which runs that were completed with `ContinueAsNew`
// automatically expire.
// If set to 0 (default), the overall expiration setting set with `WithAutoExpiration` will be used.
func WithAutoExpirationContinueAsNew(expireContinuedAsNewRunsAfter time.Duration) RedisBackendOption {
	return func(o *RedisOptions) {
		o.AutoExpirationContinueAsNew = expireContinuedAsNewRunsAfter
	}
}

func WithBackendOptions(opts ...backend.BackendOption) RedisBackendOption {
	return func(o *RedisOptions) {
		for _, opt := range opts {
			opt(o.Options)
		}
	}
}
