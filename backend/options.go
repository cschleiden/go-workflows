package backend

import (
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/metrics"
	mi "github.com/cschleiden/go-workflows/internal/metrics"
	"github.com/cschleiden/go-workflows/internal/propagators"
	"github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type Options struct {
	Logger *slog.Logger

	Metrics metrics.Client

	TracerProvider trace.TracerProvider

	// Converter is the converter to use for serializing and deserializing inputs and results. If not explicitly set
	// converter.DefaultConverter is used.
	Converter converter.Converter

	// ContextPropagators is a list of context propagators to use for passing context into workflows and activities.
	ContextPropagators []workflow.ContextPropagator

	StickyTimeout time.Duration

	// WorkflowLockTimeout determines how long a workflow task can be locked for. If the workflow task is not completed
	// by that timeframe, it's considered abandoned and another worker might pick it up.
	//
	// For long running workflow tasks, combine this with heartbearts.
	WorkflowLockTimeout time.Duration

	// ActivityLockTimeout determines how long an activity task can be locked for. If the activity task is not completed
	// by that timeframe, it's considered abandoned and another worker might pick it up
	ActivityLockTimeout time.Duration

	// RemoveContinuedAsNewInstances determines whether instances that were completed using ContinueAsNew should be
	// removed immediately, including their history. If set to false, the instance will be removed after the configured
	// retention period or never.
	RemoveContinuedAsNewInstances bool

	// MaxHistorySize is the maximum size of a workflow history. If a workflow exceeds this size, it will be failed.
	MaxHistorySize int64

	// WorkerName allows setting a custom worker name. If not set, backends will generate a default name.
	WorkerName string
}

var DefaultOptions Options = Options{
	StickyTimeout:       30 * time.Second,
	WorkflowLockTimeout: time.Minute,
	ActivityLockTimeout: time.Minute * 2,

	Logger:         slog.Default(),
	Metrics:        mi.NewNoopMetricsClient(),
	TracerProvider: noop.NewTracerProvider(),
	Converter:      converter.DefaultConverter,

	ContextPropagators: []workflow.ContextPropagator{&propagators.TracingContextPropagator{}},

	RemoveContinuedAsNewInstances: false,

	MaxHistorySize: 10_000,
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

func WithRemoveContinuedAsNewInstances() BackendOption {
	return func(o *Options) {
		o.RemoveContinuedAsNewInstances = true
	}
}

func WithMaxHistorySize(size int64) BackendOption {
	return func(o *Options) {
		o.MaxHistorySize = size
	}
}

func WithWorkerName(workerName string) BackendOption {
	return func(o *Options) {
		o.WorkerName = workerName
	}
}

func ApplyOptions(opts ...BackendOption) *Options {
	options := DefaultOptions

	for _, opt := range opts {
		opt(&options)
	}

	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	return &options
}
