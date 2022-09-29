package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/log"
	"github.com/cschleiden/go-workflows/metrics"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
)

//go:embed schema.sql
var schema string

func NewPostgresBackendFromExisting(db *sql.DB, opts ...backend.BackendOption) *postgresBackend {
	return &postgresBackend{
		db:         db,
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
		options:    backend.ApplyOptions(opts...),
	}
}

func NewPostgresBackend(host string, port int, user, password, database string, disableSsl bool, opts ...backend.BackendOption) *postgresBackend {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, database)

	if disableSsl {
		dsn = dsn + " sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	if _, err := db.Exec(schema); err != nil {
		panic(fmt.Errorf("initializing database: %w", err))
	}

	if err := db.Close(); err != nil {
		panic(err)
	}

	db, err = sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	return &postgresBackend{
		db:         db,
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
		options:    backend.ApplyOptions(opts...),
	}
}

type postgresBackend struct {
	db         *sql.DB
	workerName string
	options    backend.Options
}

// CreateWorkflowInstance creates a new workflow instance
func (b *postgresBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Create workflow instance
	if err := createInstance(ctx, tx, instance, event.Attributes.(*history.ExecutionStartedAttributes).Metadata, false); err != nil {
		return err
	}

	// Initial history is empty, store only new events
	if err := insertPendingEvents(ctx, tx, instance.InstanceID, []history.Event{event}); err != nil {
		return fmt.Errorf("inserting new event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("creating workflow instance: %w", err)
	}

	return nil
}

func (b *postgresBackend) Logger() log.Logger {
	return b.options.Logger
}

func (b *postgresBackend) Tracer() trace.Tracer {
	return b.options.TracerProvider.Tracer(backend.TracerName)
}

func (b *postgresBackend) Metrics() metrics.Client {
	return b.options.Metrics.WithTags(metrics.Tags{metrickeys.Backend: "postgres"})
}

func (b *postgresBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	instanceID := instance.InstanceID

	// Cancel workflow instance
	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, "SELECT 1 FROM gwf.instances WHERE instance_id = $1 LIMIT 1", instanceID)
	if err := res.Scan(new(int)); err != nil {
		if err == sql.ErrNoRows {
			return backend.ErrInstanceNotFound
		}

		return err
	}

	if err := insertPendingEvents(ctx, tx, instanceID, []history.Event{*event}); err != nil {
		return fmt.Errorf("inserting cancellation event: %w", err)
	}

	return tx.Commit()
}

func (b *postgresBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]history.Event, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var historyEvents *sql.Rows
	if lastSequenceID != nil {
		historyEvents, err = tx.QueryContext(
			ctx,
			"SELECT event_id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at FROM gwf.history WHERE instance_id = $1 AND sequence_id > $2 ORDER BY sequence_id",
			instance.InstanceID,
			*lastSequenceID,
		)
	} else {
		historyEvents, err = tx.QueryContext(
			ctx,
			"SELECT event_id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at FROM gwf.history WHERE instance_id = $1 ORDER BY sequence_id",
			instance.InstanceID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	h := make([]history.Event, 0)

	for historyEvents.Next() {
		var instanceID string
		var attributes []byte

		historyEvent := history.Event{}

		if err := historyEvents.Scan(
			&historyEvent.ID,
			&historyEvent.SequenceID,
			&instanceID,
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

	return h, nil
}

func (b *postgresBackend) GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error) {
	row := b.db.QueryRowContext(
		ctx,
		"SELECT completed_at FROM gwf.instances WHERE instance_id = $1 AND execution_id = $2",
		instance.InstanceID,
		instance.ExecutionID,
	)

	var completedAt sql.NullTime
	if err := row.Scan(&completedAt); err != nil {
		if err == sql.ErrNoRows {
			return core.WorkflowInstanceStateActive, backend.ErrInstanceNotFound
		}
	}

	if completedAt.Valid {
		return core.WorkflowInstanceStateFinished, nil
	}

	return core.WorkflowInstanceStateActive, nil
}

func createInstance(ctx context.Context, tx *sql.Tx, wfi *workflow.Instance, metadata *workflow.Metadata, ignoreDuplicate bool) error {
	var parentInstanceID *string
	var parentEventID *int64
	if wfi.SubWorkflow() {
		i := wfi.ParentInstanceID
		parentInstanceID = &i

		n := wfi.ParentEventID
		parentEventID = &n
	}

	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	metadataString := string(metadataJson)

	res, err := tx.ExecContext(
		ctx,
		"INSERT INTO gwf.instances (instance_id, execution_id, parent_instance_id, parent_schedule_event_id, metadata) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING",
		wfi.InstanceID,
		wfi.ExecutionID,
		parentInstanceID,
		parentEventID,
		metadataString,
	)
	if err != nil {
		return fmt.Errorf("inserting workflow instance: %w", err)
	}

	if !ignoreDuplicate {
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if rows != 1 {
			return backend.ErrInstanceAlreadyExists
		}
	}

	return nil
}

// SignalWorkflow signals a running workflow instance
func (b *postgresBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, "SELECT 1 FROM gwf.instances WHERE instance_id = $1 LIMIT 1", instanceID)
	if err := res.Scan(nil); err == sql.ErrNoRows {
		return backend.ErrInstanceNotFound
	}

	if err := insertPendingEvents(ctx, tx, instanceID, []history.Event{event}); err != nil {
		return fmt.Errorf("inserting signal event: %w", err)
	}

	return tx.Commit()
}

// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
func (b *postgresBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next workflow task by finding an unlocked instance with new events to process.
	now := time.Now()
	row := tx.QueryRowContext(
		ctx,
		`SELECT i.id, i.instance_id, i.execution_id, i.parent_instance_id, i.parent_schedule_event_id, i.metadata, i.sticky_until
			FROM gwf.instances i
			INNER JOIN gwf.pending_events pe ON i.instance_id = pe.instance_id
			WHERE
				i.completed_at IS NULL
				AND (pe.visible_at IS NULL OR pe.visible_at <= $1)
				AND (i.locked_until IS NULL OR i.locked_until < $2)
				AND (i.sticky_until IS NULL OR i.sticky_until < $3 OR i.worker = $4)
			LIMIT 1
			FOR UPDATE SKIP LOCKED`,
		now,          // event.visible_at
		now,          // locked_until
		now,          // sticky_until
		b.workerName, // worker
	)

	var id int
	var instanceID, executionID string
	var parentInstanceID *string
	var parentEventID *int64
	var metadataJson sql.NullString
	var stickyUntil *time.Time
	if err := row.Scan(&id, &instanceID, &executionID, &parentInstanceID, &parentEventID, &metadataJson, &stickyUntil); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("scanning workflow instance: %w", err)
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE gwf.instances i
			SET locked_until = $1, worker = $2
			WHERE id = $3`,
		now.Add(b.options.WorkflowLockTimeout),
		b.workerName,
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
	if parentInstanceID != nil {
		wfi = core.NewSubWorkflowInstance(instanceID, executionID, *parentInstanceID, *parentEventID)
	} else {
		wfi = core.NewWorkflowInstance(instanceID, executionID)
	}

	var metadata *core.WorkflowMetadata
	if metadataJson.Valid {
		if err := json.Unmarshal([]byte(metadataJson.String), &metadata); err != nil {
			return nil, fmt.Errorf("parsing workflow metadata: %w", err)
		}
	}

	t := &task.Workflow{
		ID:                    wfi.InstanceID,
		WorkflowInstance:      wfi,
		WorkflowInstanceState: core.WorkflowInstanceStateActive,
		Metadata:              metadata,
		NewEvents:             []history.Event{},
	}

	// Get new events
	events, err := tx.QueryContext(
		ctx,
		"SELECT event_id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at FROM gwf.pending_events WHERE instance_id = $1 AND (visible_at IS NULL OR visible_at <= $2) ORDER BY id",
		instanceID,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("getting new events: %w", err)
	}

	for events.Next() {
		var instanceID string
		var attributes []byte

		historyEvent := history.Event{}

		if err := events.Scan(
			&historyEvent.ID,
			&historyEvent.SequenceID,
			&instanceID,
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

	// Return if there aren't any new events
	if len(t.NewEvents) == 0 {
		return nil, nil
	}

	// Get most recent sequence id
	row = tx.QueryRowContext(ctx, "SELECT sequence_id FROM gwf.history WHERE instance_id = $1 ORDER BY id DESC LIMIT 1", instanceID)
	if err := row.Scan(
		&t.LastSequenceID,
	); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}
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
func (b *postgresBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *task.Workflow,
	instance *workflow.Instance,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []history.Event,
	workflowEvents []history.WorkflowEvent,
) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unlock instance, but keep it sticky to the current worker
	var completedAt *time.Time
	if state == core.WorkflowInstanceStateFinished {
		t := time.Now()
		completedAt = &t
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE gwf.instances SET locked_until = NULL, sticky_until = $1, completed_at = $2 WHERE instance_id = $3 AND execution_id = $4 AND worker = $5
		AND locked_until IS NOT NULL`,
		time.Now().Add(b.options.StickyTimeout),
		completedAt,
		instance.InstanceID,
		instance.ExecutionID,
		b.workerName,
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
		args := make([]string, 0, len(executedEvents))
		for _, e := range executedEvents {
			args = append(args, e.ID)
		}
		eventIds := pq.Array(args)

		if _, err := tx.ExecContext(
			ctx,
			"DELETE FROM gwf.pending_events WHERE instance_id = $1 AND event_id = ANY($2::text[])",
			instance.InstanceID,
			eventIds,
		); err != nil {
			return fmt.Errorf("deleting handled new events: %w", err)
		}
	}

	// Insert new events generated during this workflow execution to the history
	if err := insertHistoryEvents(ctx, tx, instance.InstanceID, executedEvents); err != nil {
		return fmt.Errorf("inserting new history events: %w", err)
	}

	// Schedule activities
	for _, e := range activityEvents {
		if err := scheduleActivity(ctx, tx, instance, e); err != nil {
			return fmt.Errorf("scheduling activity: %w", err)
		}
	}

	// Timer events
	if err := insertPendingEvents(ctx, tx, instance.InstanceID, timerEvents); err != nil {
		return fmt.Errorf("scheduling timers: %w", err)
	}

	for _, event := range executedEvents {
		switch event.Type {
		case history.EventType_TimerCanceled:
			if err := removeFutureEvent(ctx, tx, instance.InstanceID, event.ScheduleEventID); err != nil {
				return fmt.Errorf("removing future event: %w", err)
			}
		}
	}

	// Insert new workflow events
	groupedEvents := history.EventsByWorkflowInstanceID(workflowEvents)

	for targetInstanceID, events := range groupedEvents {
		for _, m := range events {
			if m.HistoryEvent.Type == history.EventType_WorkflowExecutionStarted {
				a := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)
				// Create new instance
				if err := createInstance(ctx, tx, m.WorkflowInstance, a.Metadata, true); err != nil {
					return err
				}

				break
			}
		}

		historyEvents := []history.Event{}
		for _, m := range events {
			historyEvents = append(historyEvents, m.HistoryEvent)
		}

		if err := insertPendingEvents(ctx, tx, targetInstanceID, historyEvents); err != nil {
			return fmt.Errorf("inserting messages: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing complete workflow transaction: %w", err)
	}

	return nil
}

func (b *postgresBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(b.options.WorkflowLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE gwf.instances SET locked_until = $1 WHERE instance_id = $2 AND execution_id = $3 AND worker = $4`,
		until,
		instance.InstanceID,
		instance.ExecutionID,
		b.workerName,
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
func (b *postgresBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next activity
	now := time.Now()
	res := tx.QueryRowContext(
		ctx,
		`SELECT activities.id, activity_id, activities.instance_id, activities.execution_id,
			instances.metadata, event_type, timestamp, schedule_event_id, attributes, visible_at
			FROM gwf.activities
				INNER JOIN gwf.instances ON activities.instance_id = instances.instance_id
			WHERE activities.locked_until IS NULL OR activities.locked_until < $1
			LIMIT 1
			FOR UPDATE SKIP LOCKED`,
		now,
	)

	var id int64
	var instanceID, executionID string
	var attributes []byte
	var metadataJson sql.NullString
	event := history.Event{}

	if err := res.Scan(
		&id, &event.ID, &instanceID, &executionID, &metadataJson, &event.Type,
		&event.Timestamp, &event.ScheduleEventID, &attributes, &event.VisibleAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("finding activity task to lock: %w", err)
	}

	var metadata *workflow.Metadata
	if err := json.Unmarshal([]byte(metadataJson.String), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	a, err := history.DeserializeAttributes(event.Type, attributes)
	if err != nil {
		return nil, fmt.Errorf("deserializing attributes: %w", err)
	}

	event.Attributes = a

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE gwf.activities SET locked_until = $1, worker = $2 WHERE id = $3`,
		now.Add(b.options.ActivityLockTimeout),
		b.workerName,
		id,
	); err != nil {
		return nil, fmt.Errorf("locking activity: %w", err)
	}

	t := &task.Activity{
		ID:               event.ID,
		WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
		Metadata:         metadata,
		Event:            event,
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

// CompleteActivityTask completes a activity task retrieved using GetActivityTask
func (b *postgresBackend) CompleteActivityTask(ctx context.Context, instance *workflow.Instance, id string, event history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove activity
	if res, err := tx.ExecContext(
		ctx,
		`DELETE FROM gwf.activities WHERE activity_id = $1 AND instance_id = $2 AND execution_id = $3 AND worker = $4`,
		id,
		instance.InstanceID,
		instance.ExecutionID,
		b.workerName,
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
	if err := insertPendingEvents(ctx, tx, instance.InstanceID, []history.Event{event}); err != nil {
		return fmt.Errorf("inserting new events for completed activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (b *postgresBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(b.options.ActivityLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE gwf.activities SET locked_until = $1 WHERE activity_id = $2 AND worker = $3`,
		until,
		activityID,
		b.workerName,
	)
	if err != nil {
		return fmt.Errorf("extending activity lock: %w", err)
	}

	if rowsAffected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("determining if activity was extended: %w", err)
	} else if rowsAffected == 0 {
		return errors.New("could not extend activity")
	}

	return tx.Commit()
}

func scheduleActivity(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, event history.Event) error {
	a, err := history.SerializeAttributes(event.Attributes)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO gwf.activities
			(activity_id, instance_id, execution_id, event_type, timestamp, schedule_event_id, attributes, visible_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		event.ID,
		instance.InstanceID,
		instance.ExecutionID,
		event.Type,
		event.Timestamp,
		event.ScheduleEventID,
		a,
		event.VisibleAt,
	)

	return err
}