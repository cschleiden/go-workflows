package backend

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/logger"
	"github.com/cschleiden/go-workflows/log"
	"go.opentelemetry.io/otel/trace"
)

type Options struct {
	Logger log.Logger

	TracerProvider trace.TracerProvider

	StickyTimeout time.Duration

	WorkflowLockTimeout time.Duration

	ActivityLockTimeout time.Duration
}

var DefaultOptions Options = Options{
	StickyTimeout:       30 * time.Second,
	WorkflowLockTimeout: time.Minute,
	ActivityLockTimeout: time.Minute * 2,

	Logger:         logger.NewDefaultLogger(),
	TracerProvider: trace.NewNoopTracerProvider(),
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

func WithTracerProvider(tp trace.TracerProvider) BackendOption {
	return func(o *Options) {
		o.TracerProvider = tp
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
