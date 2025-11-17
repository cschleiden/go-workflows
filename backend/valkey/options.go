package valkey

import (
	"time"

	"github.com/cschleiden/go-workflows/backend"
)

type ValkeyOptions struct {
	*backend.Options

	BlockTimeout time.Duration

	AutoExpiration              time.Duration
	AutoExpirationContinueAsNew time.Duration

	KeyPrefix string
}

type ValkeyBackendOption func(*ValkeyOptions)

// WithKeyPrefix sets the prefix for all keys used in the Valkey backend.
func WithKeyPrefix(prefix string) ValkeyBackendOption {
	return func(o *ValkeyOptions) {
		o.KeyPrefix = prefix
	}
}

// WithBlockTimeout sets the timeout for blocking operations like dequeuing a workflow or activity task
func WithBlockTimeout(timeout time.Duration) ValkeyBackendOption {
	return func(o *ValkeyOptions) {
		o.BlockTimeout = timeout
	}
}

// WithAutoExpiration sets the duration after which finished runs will expire from the data store.
// If set to 0 (default), runs will never expire and need to be manually removed.
func WithAutoExpiration(expireFinishedRunsAfter time.Duration) ValkeyBackendOption {
	return func(o *ValkeyOptions) {
		o.AutoExpiration = expireFinishedRunsAfter
	}
}

// WithAutoExpirationContinueAsNew sets the duration after which runs that were completed with `ContinueAsNew`
// automatically expire.
// If set to 0 (default), the overall expiration setting set with `WithAutoExpiration` will be used.
func WithAutoExpirationContinueAsNew(expireContinuedAsNewRunsAfter time.Duration) ValkeyBackendOption {
	return func(o *ValkeyOptions) {
		o.AutoExpirationContinueAsNew = expireContinuedAsNewRunsAfter
	}
}

func WithBackendOptions(opts ...backend.BackendOption) ValkeyBackendOption {
	return func(o *ValkeyOptions) {
		for _, opt := range opts {
			opt(o.Options)
		}
	}
}
