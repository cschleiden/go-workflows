package sqlite

import (
	"github.com/cschleiden/go-workflows/backend"
)

type options struct {
	*backend.Options

	// ApplyMigrations automatically applies database migrations on startup.
	ApplyMigrations bool

	// MigrationsTable is the table used to track applied migrations. If empty,
	// golang-migrate uses its default table.
	MigrationsTable string
}

type option func(*options)

// WithApplyMigrations automatically applies database migrations on startup.
func WithApplyMigrations(applyMigrations bool) option {
	return func(o *options) {
		o.ApplyMigrations = applyMigrations
	}
}

// WithMigrationsTable sets the table used to track applied migrations.
func WithMigrationsTable(migrationsTable string) option {
	return func(o *options) {
		o.MigrationsTable = migrationsTable
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
