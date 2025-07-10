package sqlite

import (
	"github.com/cschleiden/go-workflows/backend"
)

type options struct {
	*backend.Options

	// ApplyMigrations automatically applies database migrations on startup.
	ApplyMigrations bool

	// WorkerName allows setting a custom worker name. If not set, a random UUID will be generated.
	WorkerName string
}

type option func(*options)

// WithWorkerName sets a custom worker name for the SQLite backend.
// If not provided, a random UUID will be generated.
func WithWorkerName(workerName string) option {
	return func(o *options) {
		o.WorkerName = workerName
	}
}

// WithApplyMigrations automatically applies database migrations on startup.
func WithApplyMigrations(applyMigrations bool) option {
	return func(o *options) {
		o.ApplyMigrations = applyMigrations
	}
}

// WithBackendOptions allows to pass generic backend options.
func WithBackendOptions(opts ...backend.BackendOption) option {
	return func(o *options) {
		for _, opt := range opts {
			opt(o.Options)
		}
	}
}
