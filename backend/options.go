package backend

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/logger"
	"github.com/cschleiden/go-workflows/log"
)

type Options struct {
	Logger log.Logger

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

func WithLogger(logger log.Logger) BackendOption {
	return func(o *Options) {
		o.Logger = logger
	}
}

func ApplyOptions(opts ...BackendOption) Options {
	options := DefaultOptions

	for _, opt := range opts {
		opt(&options)
	}

	if options.Logger == nil {
		options.Logger = logger.NewDefaultLogger()
	}

	return options
}
