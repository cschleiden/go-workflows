package postgres

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel/trace"
)

//go:embed db/migrations/*.sql
var migrationsFS embed.FS

func NewPostgresBackend(host string, port int, user, password, database string, opts ...option) *postgresBackend {
	options := &options{
		Options:         backend.ApplyOptions(),
		ApplyMigrations: true,
	}

	for _, opt := range opts {
		opt(options)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, database)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}

	if options.PostgresOptions != nil {
		options.PostgresOptions(db)
	}

	b := &postgresBackend{
		dsn:            dsn,
		db:             db,
		workerName:     getWorkerName(options),
		options:        options,
		ownsConnection: true,
	}

	if options.ApplyMigrations {
		if err := b.Migrate(); err != nil {
			panic(err)
		}
	}

	return b
}

// NewPostgresBackendWithDB creates a new Postgres backend using an existing database connection.
// When using this constructor, the backend will not close the database connection when Close() is called.
// Migrations can still be applied using WithApplyMigrations(true) as Postgres does not require
// special connection settings for migrations.
func NewPostgresBackendWithDB(db *sql.DB, opts ...option) *postgresBackend {
	options := &options{
		Options:         backend.ApplyOptions(),
		ApplyMigrations: false,
	}

	for _, opt := range opts {
		opt(options)
	}

	b := &postgresBackend{
		dsn:            "",
		db:             db,
		workerName:     getWorkerName(options),
		options:        options,
		ownsConnection: false,
	}

	if options.ApplyMigrations {
		if err := b.Migrate(); err != nil {
			panic(err)
		}
	}

	return b
}

type postgresBackend struct {
	dsn            string
	db             *sql.DB
	workerName     string
	options        *options
	ownsConnection bool
}

func (pb *postgresBackend) FeatureSupported(feature backend.Feature) bool {
	return true
}

func (pb *postgresBackend) Close() error {
	if !pb.ownsConnection {
		return nil
	}
	return pb.db.Close()
}

// Migrate applies any pending database migrations.
func (pb *postgresBackend) Migrate() error {
	var db *sql.DB
	var needsClose bool

	if pb.dsn != "" {
		var err error
		db, err = sql.Open("pgx", pb.dsn)
		if err != nil {
			return fmt.Errorf("opening schema database: %w", err)
		}
		needsClose = true
	} else {
		db = pb.db
		needsClose = false
	}

	dbi, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}

	migrations, err := iofs.New(migrationsFS, "db/migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", migrations, "postgres", dbi)
	if err != nil {
		return fmt.Errorf("creating migration: %w", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("running migrations: %w", err)
		}
	}

	if needsClose {
		if err := db.Close(); err != nil {
			return fmt.Errorf("closing schema database: %w", err)
		}
	}

	return nil
}

func (pb *postgresBackend) Tracer() trace.Tracer {
	return pb.options.TracerProvider.Tracer(backend.TracerName)
}

func (pb *postgresBackend) Metrics() metrics.Client {
	return pb.options.Metrics.WithTags(metrics.Tags{metrickeys.Backend: "postgres"})
}

func (pb *postgresBackend) Options() *backend.Options {
	return pb.options.Options
}

func (pb *postgresBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	a := event.Attributes.(*history.ExecutionStartedAttributes)

	// Create workflow instance
	if err := createInstance(ctx, tx, a.Queue, instance, a.Metadata); err != nil {
		return err
	}

	// Initial history is empty, store only new events
	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting new event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("creating workflow instance: %w", err)
	}

	return nil
}

func (pb *postgresBackend) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	tx, err := pb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := pb.removeWorkflowInstance(ctx, instance, tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (pb *postgresBackend) removeWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance, tx *sql.Tx) error {
	row := tx.QueryRowContext(ctx, "SELECT state FROM instances WHERE instance_id = $1 AND execution_id = $2 LIMIT 1", instance.InstanceID, instance.ExecutionID)
	var state core.WorkflowInstanceState
	if err := row.Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return backend.ErrInstanceNotFound
		}
		return err
	}

	if state == core.WorkflowInstanceStateActive {
		return backend.ErrInstanceNotFinished
	}

	// Delete from instances and history tables
	if _, err := tx.ExecContext(ctx, "DELETE FROM instances WHERE instance_id = $1 AND execution_id = $2", instance.InstanceID, instance.ExecutionID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM history WHERE instance_id = $1 AND execution_id = $2", instance.InstanceID, instance.ExecutionID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM attributes WHERE instance_id = $1 AND execution_id = $2", instance.InstanceID, instance.ExecutionID); err != nil {
		return err
	}

	return nil
}

func (pb *postgresBackend) RemoveWorkflowInstances(ctx context.Context, options ...backend.RemovalOption) error {
	ro := backend.DefaultRemovalOptions
	for _, opt := range options {
		opt(&ro)
	}

	rows, err := pb.db.QueryContext(ctx, "SELECT instance_id, execution_id FROM instances WHERE completed_at < $1", ro.FinishedBefore)
	if err != nil {
		return err
	}
	defer rows.Close()

	pairs := make([]struct {
		instanceID  string
		executionID string
	}, 0)
	for rows.Next() {
		var id, executionID string
		if err := rows.Scan(&id, &executionID); err != nil {
			return err
		}
		pairs = append(pairs, struct {
			instanceID  string
			executionID string
		}{
			instanceID:  id,
			executionID: executionID,
		})
	}

	if rows.Err() != nil {
		return rows.Err()
	}

	pgPairedPlaceholders := func(startIdx, pairCount int) string {
		placeholders := make([]string, pairCount)
		for i := 0; i < pairCount; i++ {
			placeholders[i] = fmt.Sprintf("($%d, $%d)", startIdx+i*2, startIdx+i*2+1)
		}
		return strings.Join(placeholders, ", ")
	}

	batchSize := ro.BatchSize
	for i := 0; i < len(pairs); i += batchSize {
		tx, err := pb.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		batch := pairs[i:min(i+batchSize, len(pairs))]
		ph := pgPairedPlaceholders(1, len(batch))

		whereCondition := fmt.Sprintf("(instance_id, execution_id) IN (%s)", ph)

		args := make([]interface{}, 0, len(batch)*2)
		for j := range batch {
			args = append(args, batch[j].instanceID)
			args = append(args, batch[j].executionID)
		}

		// Delete from instances, history and attributes tables
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM instances WHERE %v", whereCondition), args...); err != nil {
			_ = tx.Rollback()
			return err
		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM history WHERE %v", whereCondition), args...); err != nil {
			_ = tx.Rollback()
			return err
		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM attributes WHERE %v", whereCondition), args...); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (pb *postgresBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Cancel workflow instance
	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, "SELECT 1 FROM instances WHERE instance_id = $1 AND execution_id = $2 LIMIT 1", instance.InstanceID, instance.ExecutionID)
	if err := res.Scan(new(int)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return backend.ErrInstanceNotFound
		}

		return err
	}

	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting cancellation event: %w", err)
	}

	return tx.Commit()
}

func (pb *postgresBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]*history.Event, error) {
	tx, err := pb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var historyEvents *sql.Rows
	if lastSequenceID != nil {
		historyEvents, err = tx.QueryContext(
			ctx,
			"SELECT h.event_id, h.sequence_id, h.event_type, h.timestamp, h.schedule_event_id, a.data, h.visible_at FROM history h JOIN attributes a ON h.event_id = a.event_id AND a.instance_id = h.instance_id AND a.execution_id = h.execution_id WHERE h.instance_id = $1 AND h.execution_id = $2 AND h.sequence_id > $3 ORDER BY h.sequence_id",
			instance.InstanceID,
			instance.ExecutionID,
			*lastSequenceID,
		)
	} else {
		historyEvents, err = tx.QueryContext(
			ctx,
			"SELECT h.event_id, h.sequence_id, h.event_type, h.timestamp, h.schedule_event_id, a.data, h.visible_at FROM history h JOIN attributes a ON h.event_id = a.event_id AND a.instance_id = h.instance_id AND a.execution_id = h.execution_id WHERE h.instance_id = $1 AND h.execution_id = $2 ORDER BY h.sequence_id",
			instance.InstanceID,
			instance.ExecutionID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	defer historyEvents.Close()

	h := make([]*history.Event, 0)

	for historyEvents.Next() {
		var attributes []byte

		historyEvent := &history.Event{}

		if err := historyEvents.Scan(
			&historyEvent.ID,
			&historyEvent.SequenceID,
			&historyEvent.Type,
			&historyEvent.Timestamp,
			&historyEvent.ScheduleEventID,
			&attributes,
			&historyEvent.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		a, err := history.DeserializeAttributes(historyEvent.Type, attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		historyEvent.Attributes = a

		h = append(h, historyEvent)
	}

	if historyEvents.Err() != nil {
		return nil, historyEvents.Err()
	}

	return h, nil
}

func (pb *postgresBackend) GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error) {
	row := pb.db.QueryRowContext(
		ctx,
		"SELECT state FROM instances WHERE instance_id = $1 AND execution_id = $2",
		instance.InstanceID,
		instance.ExecutionID,
	)

	var state core.WorkflowInstanceState
	if err := row.Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.WorkflowInstanceStateActive, backend.ErrInstanceNotFound
		}
		return core.WorkflowInstanceStateActive, fmt.Errorf("scanning instance state: %w", err)
	}

	return state, nil
}

func createInstance(ctx context.Context, tx *sql.Tx, queue workflow.Queue, wfi *workflow.Instance, metadata *workflow.Metadata) error {
	// Check for existing instance
	err := tx.QueryRowContext(
		ctx,
		"SELECT 1 FROM instances WHERE instance_id = $1 AND state = $2 LIMIT 1",
		wfi.InstanceID,
		core.WorkflowInstanceStateActive).
		Scan(new(int))
	if err == nil {
		return backend.ErrInstanceAlreadyExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var parentInstanceID, parentExecutionID *string
	var parentEventID *int64
	if wfi.SubWorkflow() {
		parentInstanceID = &wfi.Parent.InstanceID
		parentExecutionID = &wfi.Parent.ExecutionID
		parentEventID = &wfi.ParentEventID
	}

	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO instances (queue, instance_id, execution_id, parent_instance_id, parent_execution_id, parent_schedule_event_id, metadata, state) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		string(queue),
		wfi.InstanceID,
		wfi.ExecutionID,
		parentInstanceID,
		parentExecutionID,
		parentEventID,
		metadataJson,
		core.WorkflowInstanceStateActive,
	)
	if err != nil {
		return fmt.Errorf("inserting workflow instance: %w", err)
	}

	return nil
}

// SignalWorkflow signals a running workflow instance
func (pb *postgresBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, "SELECT execution_id FROM instances WHERE instance_id = $1 AND state = $2 LIMIT 1", instanceID, core.WorkflowInstanceStateActive)
	var executionID string
	err = res.Scan(&executionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return backend.ErrInstanceNotFound
		}
		return fmt.Errorf("scanning event: %w", err)
	}

	instance := core.NewWorkflowInstance(instanceID, executionID)

	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting signal event: %w", err)
	}

	return tx.Commit()
}

func (pb *postgresBackend) PrepareWorkflowQueues(ctx context.Context, queues []workflow.Queue) error {
	return nil
}

func (pb *postgresBackend) PrepareActivityQueues(ctx context.Context, queues []workflow.Queue) error {
	return nil
}

// GetWorkflowTask returns a pending workflow task or nil if there are no pending workflow executions
func (pb *postgresBackend) GetWorkflowTask(ctx context.Context, queues []workflow.Queue) (*backend.WorkflowTask, error) {
	if len(queues) == 0 {
		return nil, errors.New("no queues provided")
	}
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	args := []any{
		core.WorkflowInstanceStateActive, // state
		now,                              // event.visible_at
		now,                              // locked_until
		now,                              // sticky_until
		pb.workerName,                    // worker
	}

	for _, q := range queues {
		args = append(args, string(q))
	}

	// Lock next workflow task by finding an unlocked instance with new events to process.
	row := tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT i.id, i.queue, i.instance_id, i.execution_id, i.parent_instance_id, i.parent_execution_id, i.parent_schedule_event_id, i.metadata, i.sticky_until
			FROM instances i
			INNER JOIN pending_events pe ON i.instance_id = pe.instance_id AND i.execution_id = pe.execution_id
			WHERE
				i.state = $1 AND i.completed_at IS NULL
				AND (pe.visible_at IS NULL OR pe.visible_at <= $2)
				AND (i.locked_until IS NULL OR i.locked_until < $3)
				AND (i.sticky_until IS NULL OR i.sticky_until < $4 OR i.worker = $5)
				AND (i.queue in (%s))
			LIMIT 1
			FOR UPDATE OF i SKIP LOCKED`, pgPlaceholders(6, len(queues))),
		args...,
	)

	var id int
	var queue, instanceID, executionID string
	var parentInstanceID, parentExecutionID sql.NullString
	var parentEventID sql.NullInt64
	var metadataJson sql.NullString
	var stickyUntil sql.NullTime
	if err := row.Scan(&id, &queue, &instanceID, &executionID, &parentInstanceID, &parentExecutionID, &parentEventID, &metadataJson, &stickyUntil); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("scanning workflow instance: %w", err)
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances i
			SET locked_until = $1, worker = $2
			WHERE id = $3`,
		now.Add(pb.options.WorkflowLockTimeout),
		pb.workerName,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("locking workflow instance: %w", err)
	}

	if affectedRows, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("locking workflow instance: %w", err)
	} else if affectedRows == 0 {
		// No instance locked?
		return nil, nil
	}

	var wfi *workflow.Instance
	if parentInstanceID.Valid && parentExecutionID.Valid && parentEventID.Valid {
		wfi = core.NewSubWorkflowInstance(instanceID, executionID, core.NewWorkflowInstance(parentInstanceID.String, parentExecutionID.String), parentEventID.Int64)
	} else {
		wfi = core.NewWorkflowInstance(instanceID, executionID)
	}

	var metadata *metadata.WorkflowMetadata
	if metadataJson.Valid {
		if err := json.Unmarshal([]byte(metadataJson.String), &metadata); err != nil {
			return nil, fmt.Errorf("parsing workflow metadata: %w", err)
		}
	}

	t := &backend.WorkflowTask{
		ID:                    wfi.InstanceID,
		WorkflowInstance:      wfi,
		WorkflowInstanceState: core.WorkflowInstanceStateActive,
		Metadata:              metadata,
		NewEvents:             []*history.Event{},
		Queue:                 workflow.Queue(queue),
	}

	// Get new events
	events, err := tx.QueryContext(
		ctx,
		"SELECT pe.event_id, pe.sequence_id, pe.event_type, pe.timestamp, pe.schedule_event_id, a.data, pe.visible_at FROM pending_events pe LEFT JOIN attributes a ON pe.instance_id = a.instance_id AND pe.execution_id = a.execution_id AND pe.event_id = a.event_id WHERE pe.instance_id = $1 AND pe.execution_id = $2 AND (pe.visible_at IS NULL OR pe.visible_at <= $3) ORDER BY pe.id",
		instanceID,
		executionID,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("getting new events: %w", err)
	}

	defer events.Close()

	for events.Next() {
		var attributes []byte

		historyEvent := &history.Event{}

		if err := events.Scan(
			&historyEvent.ID,
			&historyEvent.SequenceID,
			&historyEvent.Type,
			&historyEvent.Timestamp,
			&historyEvent.ScheduleEventID,
			&attributes,
			&historyEvent.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		a, err := history.DeserializeAttributes(historyEvent.Type, attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		historyEvent.Attributes = a

		t.NewEvents = append(t.NewEvents, historyEvent)
	}

	if events.Err() != nil {
		return nil, events.Err()
	}

	// Return if there aren't any new events
	if len(t.NewEvents) == 0 {
		return nil, nil
	}

	// Get most recent sequence id
	var lastSequenceID sql.NullInt64
	row = tx.QueryRowContext(ctx, "SELECT MAX(sequence_id) FROM history WHERE instance_id = $1 AND execution_id = $2", instanceID, executionID)
	if err := row.Scan(
		&lastSequenceID,
	); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}
	}

	if lastSequenceID.Valid {
		t.LastSequenceID = lastSequenceID.Int64
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

// CompleteWorkflowTask completes a workflow task retrieved using GetWorkflowTask
//
// This checkpoints the execution. events are new events from the last workflow execution
// which will be added to the workflow instance history. workflowEvents are new events for the
// completed or other workflow instances.
func (pb *postgresBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *backend.WorkflowTask,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []*history.WorkflowEvent,
) error {
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	instance := task.WorkflowInstance

	// Unlock instance, but keep it sticky to the current worker
	var completedAt *time.Time
	if state == core.WorkflowInstanceStateContinuedAsNew || state == core.WorkflowInstanceStateFinished {
		t := time.Now()
		completedAt = &t
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = NULL, sticky_until = $1, completed_at = $2, state = $3 WHERE instance_id = $4 AND execution_id = $5 AND worker = $6 AND locked_until IS NOT NULL`,
		time.Now().Add(pb.options.StickyTimeout),
		completedAt,
		state,
		instance.InstanceID,
		instance.ExecutionID,
		pb.workerName,
	)
	if err != nil {
		return fmt.Errorf("unlocking instance: %w", err)
	}

	changedRows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking for unlocked workflow instances: %w", err)
	} else if changedRows != 1 {
		return errors.New("could not find workflow instance to unlock")
	}

	// Remove handled events from task
	if len(executedEvents) > 0 {
		args := make([]interface{}, 0, len(executedEvents)+2)
		args = append(args, instance.InstanceID, instance.ExecutionID)
		for _, e := range executedEvents {
			args = append(args, e.ID)
		}

		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf(`DELETE FROM pending_events WHERE instance_id = $1 AND execution_id = $2 AND event_id IN (%s)`, pgPlaceholders(3, len(executedEvents))),
			args...,
		); err != nil {
			return fmt.Errorf("deleting handled new events: %w", err)
		}
	}

	// Insert new events generated during this workflow execution to the history
	if err := insertHistoryEvents(ctx, tx, instance, executedEvents); err != nil {
		return fmt.Errorf("inserting new history events: %w", err)
	}

	// Schedule activities
	for _, e := range activityEvents {
		a := e.Attributes.(*history.ActivityScheduledAttributes)
		queue := a.Queue
		if queue == "" {
			// Default to workflow queue
			queue = task.Queue
		}

		if err := scheduleActivity(ctx, tx, queue, instance, e); err != nil {
			return fmt.Errorf("scheduling activity: %w", err)
		}
	}

	// Timer events
	if err := insertPendingEvents(ctx, tx, instance, timerEvents); err != nil {
		return fmt.Errorf("scheduling timers: %w", err)
	}

	for _, event := range executedEvents {
		switch event.Type {
		case history.EventType_TimerCanceled:
			if err := removeFutureEvent(ctx, tx, instance, event.ScheduleEventID); err != nil {
				return fmt.Errorf("removing future event: %w", err)
			}
		}
	}

	// Insert new workflow events
	groupedEvents := history.EventsByWorkflowInstance(workflowEvents)

	for targetInstance, events := range groupedEvents {
		// Are we creating a new sub-workflow instance?
		m := events[0]
		if m.HistoryEvent.Type == history.EventType_WorkflowExecutionStarted {
			a := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)

			queue := a.Queue
			if queue == "" {
				queue = task.Queue
			}

			// Create new instance
			if err := createInstance(ctx, tx, queue, m.WorkflowInstance, a.Metadata); err != nil {
				if errors.Is(err, backend.ErrInstanceAlreadyExists) {
					if err := insertPendingEvents(ctx, tx, instance, []*history.Event{
						history.NewPendingEvent(time.Now(), history.EventType_SubWorkflowFailed, &history.SubWorkflowFailedAttributes{
							Error: workflowerrors.FromError(backend.ErrInstanceAlreadyExists),
						}, history.ScheduleEventID(m.WorkflowInstance.ParentEventID)),
					}); err != nil {
						return fmt.Errorf("inserting sub-workflow failed event: %w", err)
					}

					continue
				}

				return fmt.Errorf("creating sub-workflow instance: %w", err)
			}
		}

		// Insert pending events for target instance
		historyEvents := []*history.Event{}
		for _, m := range events {
			historyEvents = append(historyEvents, m.HistoryEvent)
		}
		if err := insertPendingEvents(ctx, tx, &targetInstance, historyEvents); err != nil {
			return fmt.Errorf("inserting messages: %w", err)
		}
	}

	if pb.options.RemoveContinuedAsNewInstances && state == core.WorkflowInstanceStateContinuedAsNew {
		if err := pb.removeWorkflowInstance(ctx, instance, tx); err != nil {
			return fmt.Errorf("removing old instance: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing complete workflow transaction: %w", err)
	}

	return nil
}

func (pb *postgresBackend) ExtendWorkflowTask(ctx context.Context, task *backend.WorkflowTask) error {
	tx, err := pb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(pb.options.WorkflowLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = $1 WHERE instance_id = $2 AND execution_id = $3 AND worker = $4`,
		until,
		task.WorkflowInstance.InstanceID,
		task.WorkflowInstance.ExecutionID,
		pb.workerName,
	)
	if err != nil {
		return fmt.Errorf("extending workflow task lock: %w", err)
	}

	if rowsAffected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("determining if workflow task was extended: %w", err)
	} else if rowsAffected == 0 {
		return errors.New("could not extend workflow task")
	}

	return tx.Commit()
}

// GetActivityTask returns a pending activity task or nil if there are no pending activities
func (pb *postgresBackend) GetActivityTask(ctx context.Context, queues []workflow.Queue) (*backend.ActivityTask, error) {
	if len(queues) == 0 {
		return nil, errors.New("no queues provided")
	}
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next activity
	now := time.Now()

	args := make([]interface{}, 0, len(queues)+1)
	args = append(args, now)

	for _, q := range queues {
		args = append(args, string(q))
	}

	res := tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT a.id, a.activity_id, a.instance_id, a.execution_id, a.queue,
			a.event_type, a.timestamp, a.schedule_event_id, at.data, a.visible_at
			FROM activities a
			JOIN attributes at ON at.event_id = a.activity_id AND at.instance_id = a.instance_id AND at.execution_id = a.execution_id
			WHERE (a.locked_until IS NULL OR a.locked_until < $1) AND a.queue IN (%s)
			LIMIT 1
			FOR UPDATE OF a SKIP LOCKED`, pgPlaceholders(2, len(queues))),
		args...,
	)

	var id int64
	var instanceID, executionID, queue string
	var attributes []byte
	event := &history.Event{}

	if err := res.Scan(
		&id, &event.ID, &instanceID, &executionID, &queue, &event.Type,
		&event.Timestamp, &event.ScheduleEventID, &attributes, &event.VisibleAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("finding activity task to lock: %w", err)
	}

	a, err := history.DeserializeAttributes(event.Type, attributes)
	if err != nil {
		return nil, fmt.Errorf("deserializing attributes: %w", err)
	}

	event.Attributes = a

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = $1, worker = $2 WHERE id = $3`,
		now.Add(pb.options.ActivityLockTimeout),
		pb.workerName,
		id,
	); err != nil {
		return nil, fmt.Errorf("locking activity: %w", err)
	}

	t := &backend.ActivityTask{
		ID:               event.ID,
		ActivityID:       event.ID,
		Queue:            workflow.Queue(queue),
		WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
		Event:            event,
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

// CompleteActivityTask completes a activity task retrieved using GetActivityTask
func (pb *postgresBackend) CompleteActivityTask(ctx context.Context, task *backend.ActivityTask, result *history.Event) error {
	tx, err := pb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove activity
	if res, err := tx.ExecContext(
		ctx,
		`DELETE FROM activities WHERE activity_id = $1 AND instance_id = $2 AND execution_id = $3 AND worker = $4 AND queue = $5`,
		task.ActivityID,
		task.WorkflowInstance.InstanceID,
		task.WorkflowInstance.ExecutionID,
		pb.workerName,
		task.Queue,
	); err != nil {
		return fmt.Errorf("completing activity: %w", err)
	} else {
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("checking for completed activity: %w", err)
		}

		if affected == 0 {
			return errors.New("could not find locked activity")
		}
	}

	// Insert new event generated during this workflow execution
	if err := insertPendingEvents(ctx, tx, task.WorkflowInstance, []*history.Event{result}); err != nil {
		return fmt.Errorf("inserting new events for completed activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (pb *postgresBackend) ExtendActivityTask(ctx context.Context, task *backend.ActivityTask) error {
	tx, err := pb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(pb.options.ActivityLockTimeout)
	_, err = tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = $1 WHERE activity_id = $2 AND worker = $3`,
		until,
		task.ActivityID,
		pb.workerName,
	)
	if err != nil {
		return fmt.Errorf("extending activity lock: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if errors.Is(err, sql.ErrTxDone) {
			return nil
		}

		return err
	}

	return nil
}

func scheduleActivity(ctx context.Context, tx *sql.Tx, queue workflow.Queue, instance *core.WorkflowInstance, event *history.Event) error {
	// Attributes are already persisted via the history, we do not need to add them again.
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO activities
			(activity_id, instance_id, execution_id, queue, event_type, timestamp, schedule_event_id, visible_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		event.ID,
		instance.InstanceID,
		instance.ExecutionID,
		string(queue),
		event.Type,
		event.Timestamp,
		event.ScheduleEventID,
		event.VisibleAt,
	)

	return err
}

// getWorkerName returns the worker name from options, or generates a UUID-based name if not set.
func getWorkerName(options *options) string {
	if options.WorkerName != "" {
		return options.WorkerName
	}
	return fmt.Sprintf("worker-%v", uuid.NewString())
}

// pgPlaceholders builds a comma-separated list of pgx-style placeholders starting at a given index.
// Example: pgPlaceholders(3, 4) => "$3,$4,$5,$6"
func pgPlaceholders(start, count int) string {
	if count <= 0 {
		return ""
	}
	ph := make([]string, count)
	for i := 0; i < count; i++ {
		ph[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(ph, ",")
}
