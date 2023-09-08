package backend

import (
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/converter"
	mi "github.com/cschleiden/go-workflows/internal/metrics"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/metrics"
	"github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel/trace"
)

type Options struct {
	Logger *slog.Logger

	Metrics metrics.Client

	TracerProvider trace.TracerProvider

	// Converter is the converter to use for serializing and deserializing inputs and results. If not explicitly set
	// converter.DefaultConverter is used.
	Converter converter.Converter

	// ContextPropagators is a list of context propagators to use for passing context into workflows and activities.
	ContextPropagators []contextpropagation.ContextPropagator

	StickyTimeout time.Duration

	// WorkflowLockTimeout determines how long a workflow task can be locked for. If the workflow task is not completed
	// by that timeframe, it's considered abandoned and another worker might pick it up.
	//
	// For long running workflow tasks, combine this with heartbearts.
	WorkflowLockTimeout time.Duration

	// ActivityLockTimeout determines how long an activity task can be locked for. If the activity task is not completed
	// by that timeframe, it's considered abandoned and another worker might pick it up
	ActivityLockTimeout time.Duration
}

var DefaultOptions Options = Options{
	StickyTimeout:       30 * time.Second,
	WorkflowLockTimeout: time.Minute,
	ActivityLockTimeout: time.Minute * 2,

	Logger:         slog.Default(),
	Metrics:        mi.NewNoopMetricsClient(),
	TracerProvider: trace.NewNoopTracerProvider(),
	Converter:      converter.DefaultConverter,

	ContextPropagators: []contextpropagation.ContextPropagator{&tracing.TracingContextPropagator{}},
}

type BackendOption func(*Options)

func WithStickyTimeout(timeout time.Duration) BackendOption {
	return func(o *Options) {
		o.StickyTimeout = timeout
	}
}

func WithLogger(logger *slog.Logger) BackendOption {
	return func(o *Options) {
		o.Logger = logger
	}
}

func WithMetrics(client metrics.Client) BackendOption {
	return func(o *Options) {
		o.Metrics = client
	}
}

func WithTracerProvider(tp trace.TracerProvider) BackendOption {
	return func(o *Options) {
		o.TracerProvider = tp
	}
}

func WithConverter(converter converter.Converter) BackendOption {
	return func(o *Options) {
		o.Converter = converter
	}
}

func WithContextPropagator(prop workflow.ContextPropagator) BackendOption {
	return func(o *Options) {
		o.ContextPropagators = append(o.ContextPropagators, prop)
	}
}

func ApplyOptions(opts ...BackendOption) Options {
	options := DefaultOptions

	for _, opt := range opts {
		opt(&options)
	}

	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	return options
}
