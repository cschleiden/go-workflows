package valkey

import (
	"time"

	"github.com/cschleiden/go-workflows/backend"
)

type Options struct {
	*backend.Options

	BlockTimeout time.Duration

	AutoExpiration              time.Duration
	AutoExpirationContinueAsNew time.Duration

	KeyPrefix string
}

type BackendOption func(*Options)

// WithKeyPrefix sets the prefix for all keys used in the Valkey backend.
func WithKeyPrefix(prefix string) BackendOption {
	return func(o *Options) {
		o.KeyPrefix = prefix
	}
}

// WithBlockTimeout sets the timeout for blocking operations like dequeuing a workflow or activity task
func WithBlockTimeout(timeout time.Duration) BackendOption {
	return func(o *Options) {
		o.BlockTimeout = timeout
	}
}

// WithAutoExpiration sets the duration after which finished runs will expire from the data store.
// If set to 0 (default), runs will never expire and need to be manually removed.
func WithAutoExpiration(expireFinishedRunsAfter time.Duration) BackendOption {
	return func(o *Options) {
		o.AutoExpiration = expireFinishedRunsAfter
	}
}

// WithAutoExpirationContinueAsNew sets the duration after which runs that were completed with `ContinueAsNew`
// automatically expire.
// If set to 0 (default), the overall expiration setting set with `WithAutoExpiration` will be used.
func WithAutoExpirationContinueAsNew(expireContinuedAsNewRunsAfter time.Duration) BackendOption {
	return func(o *Options) {
		o.AutoExpirationContinueAsNew = expireContinuedAsNewRunsAfter
	}
}

func WithBackendOptions(opts ...backend.BackendOption) BackendOption {
	return func(o *Options) {
		for _, opt := range opts {
			opt(o.Options)
		}
	}
}
