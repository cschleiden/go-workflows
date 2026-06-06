package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const testUser = "root"
const testPassword = "root"

// Creating and dropping databases is inefficient but gives full isolation for tests.
// For future optimization consider: transactional tests (pgx + rollback),
// or TRUNCATE between tests within a single database

func Test_PostgresBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Connect to default "postgres" database to create a new one
		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			panic(err)
		}
		defer adminDB.Close()

		dbName = "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := adminDB.Exec("CREATE DATABASE " + dbName); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		options = append(options, backend.WithStickyTimeout(0))

		// Create backend pointing to the newly created database
		return NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		// Close backend DB first
		if err := b.(*postgresBackend).db.Close(); err != nil {
			panic(err)
		}

		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			panic(err)
		}
		defer adminDB.Close()

		if _, err := adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)"); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}
	})
}

func TestPostgresBackendE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			panic(err)
		}
		defer adminDB.Close()

		dbName = "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := adminDB.Exec("CREATE DATABASE " + dbName); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		options = append(options, backend.WithStickyTimeout(0))

		return NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		if err := b.(*postgresBackend).db.Close(); err != nil {
			panic(err)
		}

		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			panic(err)
		}
		defer adminDB.Close()

		if _, err := adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)"); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}
	})
}

var _ test.TestBackend = (*postgresBackend)(nil)

// GetFutureEvents returns all pending events that have a non-null visible_at (timers / scheduled future events).
func (pb *postgresBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	tx, err := pb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// attributes now lives in the separate attributes table (joined by event_id + instance_id + execution_id)
	rows, err := tx.QueryContext(ctx,
		`SELECT 
            pe.event_id,
            pe.sequence_id,
            pe.event_type,
            pe.timestamp,
            pe.schedule_event_id,
            a.data,
            pe.visible_at
         FROM pending_events pe
         JOIN attributes a 
           ON a.event_id = pe.event_id 
          AND a.instance_id = pe.instance_id 
          AND a.execution_id = pe.execution_id
         WHERE pe.visible_at IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("querying future events: %w", err)
	}
	defer rows.Close()

	var future []*history.Event
	for rows.Next() {
		var raw []byte
		e := &history.Event{}
		if err := rows.Scan(
			&e.ID,
			&e.SequenceID,
			&e.Type,
			&e.Timestamp,
			&e.ScheduleEventID,
			&raw,
			&e.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning future event: %w", err)
		}

		attr, err := history.DeserializeAttributes(e.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}
		e.Attributes = attr

		future = append(future, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return future, nil
}

func Test_PostgresBackend_WorkerName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Run("DefaultWorkerName", func(t *testing.T) {
		options := &options{
			Options: backend.ApplyOptions(),
		}
		workerName := getWorkerName(options)

		if !strings.HasPrefix(workerName, "worker-") {
			t.Errorf("Expected worker name to start with 'worker-', got: %s", workerName)
		}
		if len(workerName) != 43 { // 7 + 36
			t.Errorf("Expected worker name length 43, got: %d", len(workerName))
		}
	})

	t.Run("CustomWorkerName", func(t *testing.T) {
		custom := "test-worker-123"
		options := &options{
			Options: backend.ApplyOptions(backend.WithWorkerName(custom)),
		}
		if got := getWorkerName(options); got != custom {
			t.Errorf("Expected '%s', got '%s'", custom, got)
		}
	})

	t.Run("EmptyWorkerNameUsesDefault", func(t *testing.T) {
		options := &options{
			Options: backend.ApplyOptions(backend.WithWorkerName("")),
		}
		workerName := getWorkerName(options)
		if !strings.HasPrefix(workerName, "worker-") {
			t.Errorf("Expected worker name to start with 'worker-', got: %s", workerName)
		}
		if len(workerName) != 43 {
			t.Errorf("Expected worker name length 43, got: %d", len(workerName))
		}
	})
}

func Test_PostgresBackend_SSLMode(t *testing.T) {
	t.Run("DefaultSSLMode", func(t *testing.T) {
		// sql.Open doesn't actually connect, so this works without a real database
		b := NewPostgresBackend("localhost", 5432, "user", "pass", "db", WithApplyMigrations(false))
		defer b.Close()

		expected := "host=localhost port=5432 user=user password=pass dbname=db sslmode=disable"
		if b.dsn != expected {
			t.Errorf("Expected DSN %q, got %q", expected, b.dsn)
		}
	})

	t.Run("CustomSSLMode", func(t *testing.T) {
		b := NewPostgresBackend("localhost", 5432, "user", "pass", "db", WithApplyMigrations(false), WithSSLMode("require"))
		defer b.Close()

		expected := "host=localhost port=5432 user=user password=pass dbname=db sslmode=require"
		if b.dsn != expected {
			t.Errorf("Expected DSN %q, got %q", expected, b.dsn)
		}
	})

	t.Run("VerifyFullSSLMode", func(t *testing.T) {
		b := NewPostgresBackend("localhost", 5432, "user", "pass", "db", WithApplyMigrations(false), WithSSLMode("verify-full"))
		defer b.Close()

		expected := "host=localhost port=5432 user=user password=pass dbname=db sslmode=verify-full"
		if b.dsn != expected {
			t.Errorf("Expected DSN %q, got %q", expected, b.dsn)
		}
	})

	t.Run("EmptySSLModeDefaultsToDisable", func(t *testing.T) {
		b := NewPostgresBackend("localhost", 5432, "user", "pass", "db", WithApplyMigrations(false), WithSSLMode(""))
		defer b.Close()

		expected := "host=localhost port=5432 user=user password=pass dbname=db sslmode=disable"
		if b.dsn != expected {
			t.Errorf("Expected DSN %q, got %q", expected, b.dsn)
		}
	})

	t.Run("WithSSLModeOption", func(t *testing.T) {
		opts := &options{
			Options: backend.ApplyOptions(),
		}
		WithSSLMode("verify-ca")(opts)

		if opts.SSLMode != "verify-ca" {
			t.Errorf("Expected SSLMode 'verify-ca', got %q", opts.SSLMode)
		}
	})
}

func Test_PostgresBackendWithDB(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Run("UsesProvidedConnection", func(t *testing.T) {
		// Create database for test
		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			t.Fatal(err)
		}

		dbName := "test_withdb_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := adminDB.Exec("CREATE DATABASE " + dbName); err != nil {
			t.Fatal(err)
		}
		defer func() {
			adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
			adminDB.Close()
		}()

		// Create our own connection to the test database
		db, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable", testUser, testPassword, dbName))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// Create backend with existing connection and apply migrations
		backend := NewPostgresBackendWithDB(db, WithApplyMigrations(true))

		// Verify the backend uses our connection
		if backend.db != db {
			t.Error("Backend should use provided db connection")
		}
		if backend.ownsConnection {
			t.Error("Backend should not own the connection")
		}

		// Close backend - should NOT close our connection
		if err := backend.Close(); err != nil {
			t.Fatal(err)
		}

		// Verify our connection is still usable
		if err := db.Ping(); err != nil {
			t.Errorf("Connection should still be open after backend.Close(): %v", err)
		}
	})

	t.Run("MigrationsDisabledByDefault", func(t *testing.T) {
		// Create database for test
		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			t.Fatal(err)
		}

		dbName := "test_withdb2_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := adminDB.Exec("CREATE DATABASE " + dbName); err != nil {
			t.Fatal(err)
		}
		defer func() {
			adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
			adminDB.Close()
		}()

		db, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable", testUser, testPassword, dbName))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// Create backend without enabling migrations
		backend := NewPostgresBackendWithDB(db)
		defer backend.Close()

		// Tables should not exist since migrations weren't applied
		_, err = db.Exec("SELECT 1 FROM instances LIMIT 1")
		if err == nil {
			t.Error("Expected error because table should not exist")
		}
	})

	t.Run("MigrationsCanBeEnabled", func(t *testing.T) {
		// Create database for test
		adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
		if err != nil {
			t.Fatal(err)
		}

		dbName := "test_withdb3_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := adminDB.Exec("CREATE DATABASE " + dbName); err != nil {
			t.Fatal(err)
		}
		defer func() {
			adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
			adminDB.Close()
		}()

		db, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable", testUser, testPassword, dbName))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// Create backend with migrations enabled
		backend := NewPostgresBackendWithDB(db, WithApplyMigrations(true))
		defer backend.Close()

		// Tables should exist
		_, err = db.Exec("SELECT 1 FROM instances LIMIT 1")
		if err != nil {
			t.Errorf("Table should exist after migrations: %v", err)
		}
	})
}
