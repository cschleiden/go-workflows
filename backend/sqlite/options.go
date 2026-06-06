package sqlite

import (
	"github.com/cschleiden/go-workflows/backend"
)

type options struct {
	*backend.Options

	// ApplyMigrations automatically applies database migrations on startup.
	ApplyMigrations bool

	// AutoVacuum runs the `PRAGMA auto_vacuum=full` when creating the connection to enable the sqlite auto-vacuum feature.
	//
	// The `VACUUM` statement is always run after enabling auto-vacuum to ensure auto-vacuum is correctly enabled and to
	// reorganize the database file and reclaim disk space.
	//
	// See
	// - https://sqlite.org/pragma.html#pragma_auto_vacuum
	// - https://sqlite.org/lang_vacuum.html.
	AutoVacuum bool
}

type option func(*options)

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

// WithAutoVacuum sets sqlite auto-vacuum to full. See options.AutoVacuum for details.
func WithAutoVacuum() option {
	return func(o *options) {
		o.AutoVacuum = true
	}
}
