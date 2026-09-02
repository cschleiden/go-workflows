package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newNotifyTestBackend creates a scratch database + backend with
// notifications enabled and returns a cleanup-registered backend.
func newNotifyTestBackend(t *testing.T, opts ...option) *postgresBackend {
	t.Helper()

	adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
	require.NoError(t, err)
	t.Cleanup(func() { adminDB.Close() })

	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
	})

	allOpts := append([]option{
		WithNotifications(true),
		WithBackendOptions(backend.WithStickyTimeout(0)),
	}, opts...)
	b := NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName, allOpts...)
	t.Cleanup(func() { b.Close() })
	return b
}

// TestNotifications_TimerFiresWithoutInsert guards the visible_at deadline:
// an event scheduled with a FUTURE visible_at (workflow timers, retry
// backoff) produces no NOTIFY when it becomes due - the notification wait
// must be bounded by the next visibility instant, not block until the
// caller's context expires.
func TestNotifications_TimerFiresWithoutInsert(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	b := newNotifyTestBackend(t)
	ctx := context.Background()

	require.NoError(t, b.PrepareWorkflowQueues(ctx, []workflow.Queue{workflow.QueueDefault}))

	// Start an instance and consume its initial task so nothing is
	// immediately visible.
	instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	started := history.NewPendingEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
		Metadata: &workflow.Metadata{},
		Queue:    workflow.QueueDefault,
	})
	require.NoError(t, b.CreateWorkflowInstance(ctx, instance, started))

	firstCtx, cancelFirst := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFirst()
	task, err := b.GetWorkflowTask(firstCtx, []workflow.Queue{workflow.QueueDefault})
	require.NoError(t, err)
	require.NotNil(t, task)

	// Schedule a timer-style event visible ~2s from now (this INSERT fires
	// a notification NOW, but nothing will fire when it becomes visible).
	fireAt := time.Now().Add(2 * time.Second)
	timerEvent := history.NewPendingEvent(time.Now(), history.EventType_TimerFired, &history.TimerFiredAttributes{
		At: fireAt,
	}, history.ScheduleEventID(1), history.VisibleAt(fireAt))

	tx, err := b.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, insertPendingEvents(ctx, tx, instance, []*history.Event{timerEvent}))
	require.NoError(t, tx.Commit())

	// Simulate task completion: pending events are only removed by
	// CompleteWorkflowTask, so drop the already-visible started event -
	// leaving ONLY the future timer - and release the instance lock so it
	// is claimable again once the timer becomes visible.
	_, err = b.db.ExecContext(ctx, "DELETE FROM pending_events WHERE visible_at IS NULL")
	require.NoError(t, err)
	_, err = b.db.ExecContext(ctx, "UPDATE instances SET locked_until = NULL, worker = NULL, sticky_until = NULL")
	require.NoError(t, err)

	// Poll like the worker does (a spurious wake from the INSERT above may
	// return one nil first). Without the visible_at deadline this loop
	// takes the full 10s context; with it, ~2s.
	start := time.Now()
	loopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var timerTask *backend.WorkflowTask
	for timerTask == nil && loopCtx.Err() == nil {
		timerTask, err = b.GetWorkflowTask(loopCtx, []workflow.Queue{workflow.QueueDefault})
		if err != nil && loopCtx.Err() != nil {
			break
		}
		require.NoError(t, err)
	}
	elapsed := time.Since(start)

	require.NotNil(t, timerTask, "timer task must be picked up without any new INSERT")
	assert.GreaterOrEqual(t, elapsed, 1500*time.Millisecond, "must not fire before visible_at")
	assert.Less(t, elapsed, 5*time.Second, "visibility deadline must bound the wait (would be 10s without it)")
}

// TestNotifications_ListenerDSN verifies the dedicated listener DSN
// (PgBouncer bypass): notifications must be DELIVERED through a listener
// connected via its own DSN, and the fallback (no listener DSN) must reuse
// the SQL DSN.
func TestNotifications_ListenerDSN(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
	require.NoError(t, err)
	defer adminDB.Close()

	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)
	defer func() { _, _ = adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)") }()

	// Dedicated URL-style listener DSN aimed straight at Postgres - distinct
	// in shape from the keyword/value SQL DSN so the assertion below proves
	// which one the listener got.
	listenerDSN := fmt.Sprintf("postgres://%s:%s@localhost:5432/%s?sslmode=disable", testUser, testPassword, dbName)
	b := NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName,
		WithNotifications(true),
		WithListenerDSN(listenerDSN),
		WithBackendOptions(backend.WithStickyTimeout(0)))
	defer b.Close()

	require.NotNil(t, b.listener)
	assert.Equal(t, listenerDSN, b.listener.dsn, "listener must use the dedicated DSN, not the SQL DSN")

	ctx := context.Background()
	require.NoError(t, b.PrepareWorkflowQueues(ctx, []workflow.Queue{workflow.QueueDefault}))

	// End-to-end: an instance created over the SQL DSN must wake the
	// listener connected over the dedicated DSN.
	instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	event := history.NewPendingEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
		Metadata: &workflow.Metadata{},
		Queue:    workflow.QueueDefault,
	})
	require.NoError(t, b.CreateWorkflowInstance(ctx, instance, event))

	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	task, err := b.GetWorkflowTask(taskCtx, []workflow.Queue{workflow.QueueDefault})
	require.NoError(t, err)
	require.NotNil(t, task, "notification must be delivered via the dedicated listener DSN")

	// Fallback: without WithListenerDSN the listener reuses the SQL DSN.
	bDefault := newNotifyTestBackend(t)
	require.NotNil(t, bDefault.listener)
	assert.Contains(t, bDefault.listener.dsn, "host=localhost", "fallback listener DSN is the SQL DSN")
}
