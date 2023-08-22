package redis

import (
	"time"

	"github.com/cschleiden/go-workflows/backend"
)

type RedisOptions struct {
	backend.Options

	BlockTimeout time.Duration

	AutoExpiration time.Duration

	KeyPrefix string
}

type RedisBackendOption func(*RedisOptions)

func WithBlockTimeout(timeout time.Duration) RedisBackendOption {
	return func(o *RedisOptions) {
		o.BlockTimeout = timeout
	}
}

func WithBackendOptions(opts ...backend.BackendOption) RedisBackendOption {
	return func(o *RedisOptions) {
		for _, opt := range opts {
			opt(&o.Options)
		}
	}
}

// WithAutoExpiration sets the duration after which finished runs will expire from the data store.
// If set to 0 (default), runs will never expire and need to be manually removed.
func WithAutoExpiration(expireFinishedRunsAfter time.Duration) RedisBackendOption {
	return func(o *RedisOptions) {
		o.AutoExpiration = expireFinishedRunsAfter
	}
}

func WithKeyPrefix(keyPrefix string) RedisBackendOption {
	return func(o *RedisOptions) {
		o.KeyPrefix = keyPrefix
	}
}
