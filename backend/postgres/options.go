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

	// ListenerDSN, when set, is the connection string used ONLY by the
	// LISTEN/NOTIFY listener. LISTEN requires a session-level connection, so
	// deployments whose regular DSN points at a transaction-pooling proxy
	// (e.g. PgBouncer in transaction mode) must aim the listener directly at
	// Postgres. Empty = reuse the backend's regular DSN.
	ListenerDSN string
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

// WithListenerDSN sets a dedicated connection string for the LISTEN/NOTIFY
// listener, bypassing transaction-pooling proxies (e.g. PgBouncer) that do
// not support LISTEN. Only used when notifications are enabled.
func WithListenerDSN(dsn string) option {
	return func(o *options) {
		o.ListenerDSN = dsn
	}
}
