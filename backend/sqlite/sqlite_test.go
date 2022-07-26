package sqlite

import (
	"context"
	"fmt"
	"testing"

	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/backend/test"
	"github.com/ticctech/go-workflows/internal/history"
)

func Test_SqliteBackend(t *testing.T) {
	test.BackendTest(t, func() test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(backend.WithStickyTimeout(0))
	}, nil)
}

func Test_EndToEndSqliteBackend(t *testing.T) {
	test.EndToEndBackendTest(t, func() test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(backend.WithStickyTimeout(0))
	}, nil)
}

var _ test.TestBackend = (*sqliteBackend)(nil)

func (sb *sqliteBackend) GetFutureEvents(ctx context.Context) ([]history.Event, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// There is no index on `visible_at`, but this is okay for test only usage.
	futureEvents, err := tx.QueryContext(
		ctx,
		"SELECT id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at FROM `pending_events` WHERE visible_at IS NOT NULL",
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
