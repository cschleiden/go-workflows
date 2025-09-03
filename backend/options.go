package backend

import (
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/metrics"
	mi "github.com/cschleiden/go-workflows/internal/metrics"
	"github.com/cschleiden/go-workflows/internal/propagators"
	"github.com/cschleiden/go-workflows/workflow"
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

// WithStickyTimeout sets the sticky timeout duration for workflow executions.
// This determines how long a worker will hold onto a workflow execution context
// before releasing it back to the task queue.
func WithStickyTimeout(timeout time.Duration) BackendOption {
	return func(o *Options) {
		o.StickyTimeout = timeout
	}
}

// WithLogger sets a custom logger for the backend.
// If not set, slog.Default() will be used.
func WithLogger(logger *slog.Logger) BackendOption {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithMetrics sets a custom metrics client for collecting workflow execution metrics.
// If not set, a no-op metrics client will be used.
func WithMetrics(client metrics.Client) BackendOption {
	return func(o *Options) {
		o.Metrics = client
	}
}

// WithTracerProvider sets a custom OpenTelemetry tracer provider for distributed tracing.
// If not set, a no-op tracer provider will be used.
func WithTracerProvider(tp trace.TracerProvider) BackendOption {
	return func(o *Options) {
		o.TracerProvider = tp
	}
}

// WithConverter sets a custom converter for serializing and deserializing workflow inputs and results.
// If not set, converter.DefaultConverter will be used.
func WithConverter(converter converter.Converter) BackendOption {
	return func(o *Options) {
		o.Converter = converter
	}
}

// WithContextPropagator adds a context propagator for passing context into workflows and activities.
// Multiple propagators can be added by calling this function multiple times.
func WithContextPropagator(prop workflow.ContextPropagator) BackendOption {
	return func(o *Options) {
		o.ContextPropagators = append(o.ContextPropagators, prop)
	}
}

// WithRemoveContinuedAsNewInstances enables immediate removal of workflow instances
// that completed using ContinueAsNew, including their history.
// If not set, instances will be removed after the configured retention period or never.
func WithRemoveContinuedAsNewInstances() BackendOption {
	return func(o *Options) {
		o.RemoveContinuedAsNewInstances = true
	}
}

// WithMaxHistorySize sets the maximum size of a workflow history in bytes.
// If a workflow exceeds this size, it will be failed to prevent unbounded growth.
func WithMaxHistorySize(size int64) BackendOption {
	return func(o *Options) {
		o.MaxHistorySize = size
	}
}

// WithWorkerName sets a custom name for this worker instance.
// If not set, backends will generate a default name based on hostname and process ID.
func WithWorkerName(workerName string) BackendOption {
	return func(o *Options) {
		o.WorkerName = workerName
	}
}

// WithWorkflowLockTimeout sets the timeout for workflow task locks. If a workflow task is not completed
// within this timeframe, it's considered abandoned and another worker might pick it up.
func WithWorkflowLockTimeout(timeout time.Duration) BackendOption {
	return func(o *Options) {
		o.WorkflowLockTimeout = timeout
	}
}

// WithActivityLockTimeout sets the timeout for activity task locks. If an activity task is not completed
// within this timeframe, it's considered abandoned and another worker might pick it up.
func WithActivityLockTimeout(timeout time.Duration) BackendOption {
	return func(o *Options) {
		o.ActivityLockTimeout = timeout
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
