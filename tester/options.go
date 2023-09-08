package tester

import (
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/converter"
)

type options struct {
	TestTimeout time.Duration
	Logger      *slog.Logger
	Converter   converter.Converter
	Propagators []contextpropagation.ContextPropagator
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

func WithContextPropagator(prop contextpropagation.ContextPropagator) WorkflowTesterOption {
	return func(o *options) {
		o.Propagators = append(o.Propagators, prop)
	}
}

func WithTestTimeout(timeout time.Duration) WorkflowTesterOption {
	return func(o *options) {
		o.TestTimeout = timeout
	}
}
