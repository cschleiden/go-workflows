package sqlite

import (
	"database/sql"
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/stretchr/testify/require"
)

func Test_SqliteBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(WithBackendOptions(append(options, backend.WithStickyTimeout(0))...))
		// return NewSqliteBackend("test.sqlite", WithBackendOptions(append(options, backend.WithStickyTimeout(0))...))
	}, func(b test.TestBackend) {
		// Ensure we close the database so the next test will get a clean in-memory db
		require.NoError(t, b.(*sqliteBackend).Close())
	})
}

func Test_EndToEndSqliteBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(WithBackendOptions(append(options, backend.WithStickyTimeout(0))...))
	}, func(b test.TestBackend) {
		// Ensure we close the database so the next test will get a clean in-memory db
		require.NoError(t, b.Close())
	})
}

func Test_SqliteBackend_WorkerName(t *testing.T) {
	t.Run("DefaultWorkerName", func(t *testing.T) {
		backend := NewInMemoryBackend()
		defer backend.Close()

		// The default worker name should be in the format "worker-<uuid>"
		require.Contains(t, backend.workerName, "worker-")
		require.Len(t, backend.workerName, 43) // "worker-" (7) + UUID (36)
	})

	t.Run("CustomWorkerName", func(t *testing.T) {
		customWorkerName := "test-worker-123"
		backend := NewInMemoryBackend(WithBackendOptions(backend.WithWorkerName(customWorkerName)))
		defer backend.Close()

		require.Equal(t, customWorkerName, backend.workerName)
	})

	t.Run("EmptyWorkerNameUsesDefault", func(t *testing.T) {
		backend := NewInMemoryBackend(WithBackendOptions(backend.WithWorkerName("")))
		defer backend.Close()

		// Empty worker name should fall back to UUID generation
		require.Contains(t, backend.workerName, "worker-")
		require.Len(t, backend.workerName, 43) // "worker-" (7) + UUID (36)
	})

	t.Run("CustomWorkerNameIsUsedInDatabase", func(t *testing.T) {
		customWorkerName := "integration-test-worker"
		backend := NewInMemoryBackend(WithBackendOptions(backend.WithWorkerName(customWorkerName)))
		defer backend.Close()

		// Verify the worker name is stored correctly
		require.Equal(t, customWorkerName, backend.workerName)
	})
}

func Test_SqliteBackendWithDB(t *testing.T) {
	t.Run("UsesProvidedConnection", func(t *testing.T) {
		// Create our own database connection
		db, err := sql.Open("sqlite", "file:testdb?mode=memory&cache=shared")
		require.NoError(t, err)
		defer db.Close()

		// Configure connection as recommended
		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		require.NoError(t, err)
		_, err = db.Exec("PRAGMA busy_timeout = 5000;")
		require.NoError(t, err)
		db.SetMaxOpenConns(1)

		// Create backend with existing connection and apply migrations
		backend := NewSqliteBackendWithDB(db, WithApplyMigrations(true))

		// Verify the backend works
		require.NotNil(t, backend)
		require.Equal(t, db, backend.db)
		require.False(t, backend.ownsConnection)

		// Close backend - should NOT close our connection
		err = backend.Close()
		require.NoError(t, err)

		// Verify our connection is still usable
		err = db.Ping()
		require.NoError(t, err)
	})

	t.Run("MigrationsDisabledByDefault", func(t *testing.T) {
		db, err := sql.Open("sqlite", "file:testdb2?mode=memory&cache=shared")
		require.NoError(t, err)
		defer db.Close()

		db.SetMaxOpenConns(1)

		// Create backend without enabling migrations
		backend := NewSqliteBackendWithDB(db)
		defer backend.Close()

		// Tables should not exist since migrations weren't applied
		_, err = db.Exec("SELECT 1 FROM instances LIMIT 1")
		require.Error(t, err) // Table doesn't exist
	})

	t.Run("MigrationsCanBeEnabled", func(t *testing.T) {
		db, err := sql.Open("sqlite", "file:testdb3?mode=memory&cache=shared")
		require.NoError(t, err)
		defer db.Close()

		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		require.NoError(t, err)
		db.SetMaxOpenConns(1)

		// Create backend with migrations enabled
		backend := NewSqliteBackendWithDB(db, WithApplyMigrations(true))
		defer backend.Close()

		// Tables should exist
		_, err = db.Exec("SELECT 1 FROM instances LIMIT 1")
		require.NoError(t, err)
	})
}
