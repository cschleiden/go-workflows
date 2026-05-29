package mysql

import (
	"database/sql"

	"github.com/cschleiden/go-workflows/backend"
)

type options struct {
	*backend.Options

	MySQLOptions func(db *sql.DB)

	// ApplyMigrations automatically applies database migrations on startup.
	ApplyMigrations bool

	// MigrationDSN is an optional DSN to use for running migrations. This is useful when
	// using NewMysqlBackendWithDB where no DSN is available. The DSN must support
	// multi-statement queries (e.g., include &multiStatements=true).
	MigrationDSN string
}

type option func(*options)

// WithApplyMigrations automatically applies database migrations on startup.
func WithApplyMigrations(applyMigrations bool) option {
	return func(o *options) {
		o.ApplyMigrations = applyMigrations
	}
}

func WithMySQLOptions(f func(db *sql.DB)) option {
	return func(o *options) {
		o.MySQLOptions = f
	}
}

// WithMigrationDSN sets the DSN to use for running migrations. This is required when
// using NewMysqlBackendWithDB with ApplyMigrations enabled. The DSN should support
// multi-statement queries.
func WithMigrationDSN(dsn string) option {
	return func(o *options) {
		o.MigrationDSN = dsn
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
