package backend

import "time"

type Options struct {
	StickyTimeout time.Duration

	WorkflowLockTimeout time.Duration

	ActivityLockTimeout time.Duration
}

var DefaultOptions Options = Options{
	StickyTimeout:       30 * time.Second,
	WorkflowLockTimeout: time.Minute,
	ActivityLockTimeout: time.Minute * 2,
}

type BackendOption func(*Options)

func WithStickyTimeout(timeout time.Duration) BackendOption {
	return func(o *Options) {
		o.StickyTimeout = timeout
	}
}

func ApplyOptions(opts ...BackendOption) Options {
	options := DefaultOptions

	for _, opt := range opts {
		opt(&options)
	}

	return options
}
