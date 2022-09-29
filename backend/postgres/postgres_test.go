package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/google/uuid"
)

const testUser = "postgres"
const testPassword = "root"

// Creating and dropping databases is terribly inefficient, but easiest for complete test isolation. For
// the future consider nested transactions, or manually TRUNCATE-ing the tables in-between tests.

func Test_PostgresBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.BackendTest(t, func() test.TestBackend {
		db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=%s password=%s sslmode=disable", testUser, testPassword))
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

		return NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName, true, backend.WithStickyTimeout(0))
	}, func(b test.TestBackend) {
		db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=%s password=%s sslmode=disable", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)"); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}
	})
}

func TestPostgresBackendE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.EndToEndBackendTest(t, func() test.TestBackend {
		db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=%s password=%s sslmode=disable", testUser, testPassword))
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

		return NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName, true, backend.WithStickyTimeout(0))
	}, func(b test.TestBackend) {

		db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=%s password=%s sslmode=disable", testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)"); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}
	})
}

var _ test.TestBackend = (*postgresBackend)(nil)

func (mb *postgresBackend) GetFutureEvents(ctx context.Context) ([]history.Event, error) {
	tx, err := mb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// There is no index on `visible_at`, but this is okay for test only usage.
	futureEvents, err := tx.QueryContext(
		ctx,
		"SELECT event_id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at FROM gwf.pending_events WHERE visible_at IS NOT NULL",
	)
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	f := make([]history.Event, 0)

	for futureEvents.Next() {
		var instanceID string
		var attributes []byte

		fe := history.Event{}

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
