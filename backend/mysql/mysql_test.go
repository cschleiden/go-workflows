package mysql

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
)

const testUser = "root"
const testPassword = "root"

// Creating and dropping databases is terribly inefficient, but easiest for complete test isolation. For
// the future consider nested transactions, or manually TRUNCATE-ing the tables in-between tests.

func Test_MysqlBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		dbName = "test_" + strings.Replace(uuid.NewString(), "-", "", -1)
		if _, err := db.Exec("CREATE DATABASE " + dbName); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}

		options = append(options, backend.WithStickyTimeout(0))

		return NewMysqlBackend("localhost", 3306, testUser, testPassword, dbName, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		if err := b.(*mysqlBackend).db.Close(); err != nil {
			panic(err)
		}

		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}
	})
}

func TestMySqlBackendE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		dbName = "test_" + strings.Replace(uuid.NewString(), "-", "", -1)
		if _, err := db.Exec("CREATE DATABASE " + dbName); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}

		options = append(options, backend.WithStickyTimeout(0))

		return NewMysqlBackend("localhost", 3306, testUser, testPassword, dbName, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		if err := b.(*mysqlBackend).db.Close(); err != nil {
			panic(err)
		}

		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}
	})
}

func TestMySqlBackendWithDB_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Create a database for testing
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		dbName = "test_with_db_e2e_" + strings.Replace(uuid.NewString(), "-", "", -1)
		if _, err := db.Exec("CREATE DATABASE " + dbName); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}

		// Create a connection to the test database
		testDB, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(localhost:3306)/%s?parseTime=true&interpolateParams=true&multiStatements=true", testUser, testPassword, dbName))
		if err != nil {
			panic(fmt.Errorf("connecting to test database: %w", err))
		}

		options = append(options, backend.WithStickyTimeout(0))

		return NewMysqlBackendWithDB(testDB, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		if err := b.Close(); err != nil {
			panic(err)
		}

		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}
	})
}

var _ test.TestBackend = (*mysqlBackend)(nil)

func (mb *mysqlBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	tx, err := mb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// There is no index on `visible_at`, but this is okay for test only usage.
	futureEvents, err := tx.QueryContext(
		ctx,
		"SELECT pe.id, pe.sequence_id, pe.instance_id, pe.execution_id, pe.event_type, pe.timestamp, pe.schedule_event_id, pe.visible_at, a.data FROM `pending_events` pe JOIN `attributes` a ON a.id = pe.id AND a.instance_id = pe.instance_id AND a.execution_id = pe.execution_id WHERE pe.visible_at IS NOT NULL",
	)
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	defer futureEvents.Close()

	f := make([]*history.Event, 0)

	for futureEvents.Next() {
		var instanceID string
		var attributes []byte

		fe := &history.Event{}

		if err := futureEvents.Scan(
			&fe.ID,
			&fe.SequenceID,
			&instanceID,
			&fe.Type,
			&fe.Timestamp,
			&fe.ScheduleEventID,
			&attributes,
			&fe.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		a, err := history.DeserializeAttributes(fe.Type, attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		fe.Attributes = a

		f = append(f, fe)
	}

	return f, nil
}

func TestNewMysqlBackendWithDB_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	t.Run("NewMysqlBackendWithDB_Basic", func(t *testing.T) {
		// Create a database for testing
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		dbName = "test_with_db_" + strings.Replace(uuid.NewString(), "-", "", -1)
		if _, err := db.Exec("CREATE DATABASE " + dbName); err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer func() {
			if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
				t.Errorf("Failed to drop database: %v", err)
			}
		}()

		// Create a connection to the test database
		testDB, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(localhost:3306)/%s?parseTime=true&interpolateParams=true&multiStatements=true", testUser, testPassword, dbName))
		if err != nil {
			t.Fatalf("Failed to connect to test database: %v", err)
		}
		defer testDB.Close()

		// Test creating backend with existing DB
		mysqlBackend := NewMysqlBackendWithDB(testDB, WithBackendOptions(backend.WithStickyTimeout(0)))
		if mysqlBackend == nil {
			t.Fatal("Expected backend to be created")
		}

		// Verify the backend has the correct DB instance
		if mysqlBackend.db != testDB {
			t.Error("Expected backend to use the provided DB instance")
		}

		// Verify DSN is empty when using existing DB
		if mysqlBackend.dsn != "" {
			t.Errorf("Expected DSN to be empty when using existing DB, got: %s", mysqlBackend.dsn)
		}

		// Test that the backend can perform basic operations
		if !mysqlBackend.FeatureSupported(backend.Feature_Expiration) {
			t.Error("Expected backend to support expiration feature")
		}

		// Close the backend
		if err := mysqlBackend.Close(); err != nil {
			t.Errorf("Failed to close backend: %v", err)
		}
	})

	t.Run("NewMysqlBackendWithDB_WithoutMigrations", func(t *testing.T) {
		// Create a database for testing
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		dbName = "test_no_migrations_" + strings.Replace(uuid.NewString(), "-", "", -1)
		if _, err := db.Exec("CREATE DATABASE " + dbName); err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer func() {
			if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
				t.Errorf("Failed to drop database: %v", err)
			}
		}()

		// Create a connection to the test database
		testDB, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(localhost:3306)/%s?parseTime=true&interpolateParams=true&multiStatements=true", testUser, testPassword, dbName))
		if err != nil {
			t.Fatalf("Failed to connect to test database: %v", err)
		}
		defer testDB.Close()

		// Test creating backend with existing DB and migrations disabled
		mysqlBackend := NewMysqlBackendWithDB(testDB,
			WithApplyMigrations(false),
			WithBackendOptions(backend.WithStickyTimeout(0)))
		if mysqlBackend == nil {
			t.Fatal("Expected backend to be created")
		}

		// Close the backend
		if err := mysqlBackend.Close(); err != nil {
			t.Errorf("Failed to close backend: %v", err)
		}
	})
}

func Test_MysqlBackend_WorkerName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Run("DefaultWorkerName", func(t *testing.T) {
		// Create a backend without specifying worker name
		// Since we can't connect to MySQL without it being available, we'll test the getWorkerName function directly
		options := &options{
			Options: backend.ApplyOptions(),
		}
		workerName := getWorkerName(options)

		// The default worker name should be in the format "worker-<uuid>"
		if !strings.Contains(workerName, "worker-") {
			t.Errorf("Expected worker name to contain 'worker-', got: %s", workerName)
		}
		if len(workerName) != 43 { // "worker-" (7) + UUID (36)
			t.Errorf("Expected worker name length to be 43, got: %d", len(workerName))
		}
	})

	t.Run("CustomWorkerName", func(t *testing.T) {
		customWorkerName := "test-worker-123"
		options := &options{
			Options: backend.ApplyOptions(backend.WithWorkerName(customWorkerName)),
		}
		workerName := getWorkerName(options)

		if workerName != customWorkerName {
			t.Errorf("Expected worker name to be '%s', got: %s", customWorkerName, workerName)
		}
	})

	t.Run("EmptyWorkerNameUsesDefault", func(t *testing.T) {
		options := &options{
			Options: backend.ApplyOptions(backend.WithWorkerName("")),
		}
		workerName := getWorkerName(options)

		// Empty worker name should fall back to UUID generation
		if !strings.Contains(workerName, "worker-") {
			t.Errorf("Expected worker name to contain 'worker-', got: %s", workerName)
		}
		if len(workerName) != 43 { // "worker-" (7) + UUID (36)
			t.Errorf("Expected worker name length to be 43, got: %d", len(workerName))
		}
	})
}
