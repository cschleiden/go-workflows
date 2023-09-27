package tester

import (
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/workflow"
)

type options struct {
	TestTimeout time.Duration
	Logger      *slog.Logger
	Converter   converter.Converter
	Propagators []workflow.ContextPropagator
}

type WorkflowTesterOption func(*options)

func WithLogger(logger *slog.Logger) WorkflowTesterOption {
	return func(o *options) {
		o.Logger = logger
	}
}

func WithConverter(converter converter.Converter) WorkflowTesterOption {
	return func(o *options) {
		o.Converter = converter
	}
}

func WithContextPropagator(prop workflow.ContextPropagator) WorkflowTesterOption {
	return func(o *options) {
		o.Propagators = append(o.Propagators, prop)
	}
}

func WithTestTimeout(timeout time.Duration) WorkflowTesterOption {
	return func(o *options) {
		o.TestTimeout = timeout
	}
}
