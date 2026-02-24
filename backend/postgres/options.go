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

	// SSLMode configures the sslmode parameter for the PostgreSQL connection.
	// Defaults to "disable" if not set.
	SSLMode string
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

// WithSSLMode configures the sslmode parameter for the PostgreSQL connection string.
// Valid values include "disable", "require", "verify-ca", "verify-full", etc.
// Defaults to "disable" if not set.
func WithSSLMode(sslmode string) option {
	return func(o *options) {
		o.SSLMode = sslmode
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
