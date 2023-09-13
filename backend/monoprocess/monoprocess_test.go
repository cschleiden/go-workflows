package monoprocess

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/internal/history"
)

func Test_MonoprocessBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		options = append(options, backend.WithStickyTimeout(0))

		return NewMonoprocessBackend(sqlite.NewInMemoryBackend(options...), 0, time.Millisecond)
	}, nil)
}

func Test_EndToEndMonoprocessBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		options = append(options, backend.WithStickyTimeout(0))

		return NewMonoprocessBackend(sqlite.NewInMemoryBackend(options...), 0, 0)
	}, nil)
}

var _ test.TestBackend = (*monoprocessBackend)(nil)

func (b *monoprocessBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	// Reflection hack to access db in private field of sqlite.sqliteBackend
	privateDbField := reflect.ValueOf(b.Backend).Elem().FieldByName("db")
	hackedDbField := reflect.NewAt(privateDbField.Type(), unsafe.Pointer(privateDbField.UnsafeAddr())).Elem()
	db := hackedDbField.Interface().(*sql.DB)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// There is no index on `visible_at`, but this is okay for test only usage.
	futureEvents, err := tx.QueryContext(
		ctx,
		"SELECT id, sequence_id, instance_id, execution_id, event_type, timestamp, schedule_event_id, attributes, visible_at FROM `pending_events` WHERE visible_at IS NOT NULL",
	)
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	f := make([]*history.Event, 0)

	for futureEvents.Next() {
		var instanceID, executionID string
		var attributes []byte

		fe := &history.Event{}

		if err := futureEvents.Scan(
			&fe.ID,
			&fe.SequenceID,
			&instanceID,
			&executionID,
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
