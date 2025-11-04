package postgres

import (
	"database/sql"

	"github.com/cschleiden/go-workflows/backend"
)

type options struct {
	*backend.Options

	PostgresOptions func(db *sql.DB)

	// ApplyMigrations automatically applies database migrations on startup.
	ApplyMigrations bool

	// EnableNotifications enables LISTEN/NOTIFY support for reactive task polling.
	// When enabled, the backend will use PostgreSQL LISTEN/NOTIFY to wake up
	// workers immediately when new tasks are available, instead of polling.
	EnableNotifications bool
}

type option func(*options)

// WithApplyMigrations automatically applies database migrations on startup.
func WithApplyMigrations(applyMigrations bool) option {
	return func(o *options) {
		o.ApplyMigrations = applyMigrations
	}
}

func WithPostgresOptions(f func(db *sql.DB)) option {
	return func(o *options) {
		o.PostgresOptions = f
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

// WithNotifications enables LISTEN/NOTIFY support for reactive task polling.
func WithNotifications(enable bool) option {
	return func(o *options) {
		o.EnableNotifications = enable
	}
}
