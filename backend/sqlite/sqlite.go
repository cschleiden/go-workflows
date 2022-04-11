package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/log"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

func NewInMemoryBackend(opts ...backend.BackendOption) backend.Backend {
	b := newSqliteBackend("file::memory:", opts...)

	b.db.SetMaxOpenConns(1)

	return b
}

func NewSqliteBackend(path string, opts ...backend.BackendOption) backend.Backend {
	return newSqliteBackend(fmt.Sprintf("file:%v", path), opts...)
}

func newSqliteBackend(dsn string, opts ...backend.BackendOption) *sqliteBackend {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		panic(err)
	}

	// Initialize database
	if _, err := db.Exec(schema); err != nil {
		panic(err)
	}

	return &sqliteBackend{
		db:         db,
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
		options:    backend.ApplyOptions(opts...),
	}
}

type sqliteBackend struct {
	db         *sql.DB
	workerName string
	options    backend.Options
}

func (sb *sqliteBackend) Logger() log.Logger {
	return sb.options.Logger
}

func (sb *sqliteBackend) CreateWorkflowInstance(ctx context.Context, m history.WorkflowEvent) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "could not start transaction")
	}
	defer tx.Rollback()

	// Create workflow instance
	if err := createInstance(ctx, tx, m.WorkflowInstance, false); err != nil {
		return err
	}

	// Initial history is empty, store only new events
	if err := insertNewEvents(ctx, tx, m.WorkflowInstance.InstanceID, []history.Event{m.HistoryEvent}); err != nil {
		return errors.Wrap(err, "could not insert new event")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	return nil
}

func createInstance(ctx context.Context, tx *sql.Tx, wfi *workflow.Instance, ignoreDuplicate bool) error {
	var parentInstanceID *string
	var parentEventID *int64
	if wfi.SubWorkflow() {
		i := wfi.ParentInstanceID
		parentInstanceID = &i

		n := wfi.ParentEventID
		parentEventID = &n
	}

	res, err := tx.ExecContext(
		ctx,
		"INSERT OR IGNORE INTO `instances` (id, execution_id, parent_instance_id, parent_schedule_event_id) VALUES (?, ?, ?, ?)",
		wfi.InstanceID,
		wfi.ExecutionID,
		parentInstanceID,
		parentEventID,
	)
	if err != nil {
		return errors.Wrap(err, "could not insert workflow instance")
	}

	if !ignoreDuplicate {
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if rows != 1 {
			return errors.New("could not insert workflow instance")
		}
	}

	return nil
}

func (sb *sqliteBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	instanceID := instance.InstanceID

	// Cancel workflow instance
	if err := insertNewEvents(ctx, tx, instanceID, []history.Event{*event}); err != nil {
		return errors.Wrap(err, "could not insert cancellation event")
	}

	// Recursively, find any sub-workflow instance to cancel
	for {
		row := tx.QueryRowContext(ctx, "SELECT id FROM `instances` WHERE parent_instance_id = ? AND completed_at IS NULL LIMIT 1", instanceID)

		var subWorkflowInstanceID string
		if err := row.Scan(&subWorkflowInstanceID); err != nil {
			if err == sql.ErrNoRows {
				// No more sub-workflow instances to cancel
				break
			}

			return errors.Wrap(err, "could not get workflow instance for cancelling")
		}

		// Cancel sub-workflow instance
		if err := insertNewEvents(ctx, tx, subWorkflowInstanceID, []history.Event{*event}); err != nil {
			return errors.Wrap(err, "could not insert cancellation event")
		}

		instanceID = subWorkflowInstanceID
	}

	return tx.Commit()
}

func (sb *sqliteBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]history.Event, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	h, err := getHistory(ctx, tx, instance.InstanceID, lastSequenceID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get workflow history")
	}

	return h, nil
}

func (s *sqliteBackend) GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (backend.WorkflowState, error) {
	row := s.db.QueryRowContext(
		ctx,
		"SELECT completed_at FROM instances WHERE id = ? AND execution_id = ?",
		instance.InstanceID,
		instance.ExecutionID,
	)

	var completedAt sql.NullTime
	if err := row.Scan(&completedAt); err != nil {
		if err == sql.ErrNoRows {
			return backend.WorkflowStateActive, errors.New("could not find workflow instance")
		}
	}

	if completedAt.Valid {
		return backend.WorkflowStateFinished, nil
	}

	return backend.WorkflowStateActive, nil
}

func (sb *sqliteBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, "SELECT 1 FROM `instances` WHERE id = ? LIMIT 1", instanceID)
	if err := res.Scan(nil); err == sql.ErrNoRows {
		return backend.ErrInstanceNotFound
	}

	if err := insertNewEvents(ctx, tx, instanceID, []history.Event{event}); err != nil {
		return errors.Wrap(err, "could not insert signal event")
	}

	return tx.Commit()
}

func (sb *sqliteBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next workflow task by finding an unlocked instance with new events to process
	// (work around missing LIMIT support in sqlite driver for UPDATE statements by using sub-query)
	now := time.Now()
	row := tx.QueryRowContext(
		ctx,
		`UPDATE instances
			SET locked_until = ?, worker = ?
			WHERE rowid = (
				SELECT rowid FROM instances i
					WHERE
						(locked_until IS NULL OR locked_until < ?)
						AND (sticky_until IS NULL OR sticky_until < ? OR worker = ?)
						AND completed_at IS NULL
						AND EXISTS (
							SELECT 1
								FROM pending_events
								WHERE instance_id = i.id AND execution_id = i.execution_id AND (visible_at IS NULL OR visible_at <= ?)
						)
					LIMIT 1
			) RETURNING id, execution_id, parent_instance_id, parent_schedule_event_id, sticky_until`,
		now.Add(sb.options.WorkflowLockTimeout), // new locked_until
		sb.workerName,
		now,           // locked_until
		now,           // sticky_until
		sb.workerName, // worker
		now,           // event.visible_at
	)

	var instanceID, executionID string
	var parentInstanceID *string
	var parentEventID *int64
	var stickyUntil *time.Time
	if err := row.Scan(&instanceID, &executionID, &parentInstanceID, &parentEventID, &stickyUntil); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, errors.Wrap(err, "could not lock workflow task")
	}

	var wfi *workflow.Instance
	if parentInstanceID != nil {
		wfi = core.NewSubWorkflowInstance(instanceID, executionID, *parentInstanceID, *parentEventID)
	} else {
		wfi = core.NewWorkflowInstance(instanceID, executionID)
	}

	t := &task.Workflow{
		ID:               wfi.InstanceID,
		WorkflowInstance: wfi,
		NewEvents:        []history.Event{},
	}

	// Get new events
	pendingEvents, err := getPendingEvents(ctx, tx, instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get pending events")
	}

	// Return if there aren't any new events
	if len(pendingEvents) == 0 {
		return nil, nil
	}

	t.NewEvents = pendingEvents

	// Get only most recent sequence ID
	// TODO: Denormalize to instances table
	row = tx.QueryRowContext(ctx, "SELECT sequence_id FROM `history` WHERE instance_id = ? ORDER BY rowid DESC LIMIT 1", instanceID)
	if err := row.Scan(&t.LastSequenceID); err != nil {
		if err != sql.ErrNoRows {
			return nil, errors.Wrap(err, "could not get most recent sequence id")
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

// CompleteWorkflowTask(ctx context.Context, instance *workflow.Instance, executedEvents []history.Event, workflowEvents []history.WorkflowEvent) error

func (sb *sqliteBackend) CompleteWorkflowTask(
	ctx context.Context,
	taskID string,
	instance *workflow.Instance,
	state backend.WorkflowState,
	executedEvents []history.Event,
	activityEvents []history.Event,
	workflowEvents []history.WorkflowEvent,
) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var completedAt *time.Time
	if state == backend.WorkflowStateFinished {
		t := time.Now()
		completedAt = &t
	}

	// Unlock instance, but keep it sticky to the current worker
	if res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = NULL, sticky_until = ?, completed_at = ? WHERE id = ? AND execution_id = ? AND worker = ?`,
		time.Now().Add(sb.options.StickyTimeout),
		completedAt,
		instance.InstanceID,
		instance.ExecutionID,
		sb.workerName,
	); err != nil {
		return errors.Wrap(err, "could not unlock workflow instance")
	} else if n, err := res.RowsAffected(); err != nil {
		return errors.Wrap(err, "could not check for unlocked workflow instances")
	} else if n != 1 {
		return errors.New("could not find workflow instance to unlock")
	}

	// Remove handled events from task
	if len(executedEvents) > 0 {
		args := make([]interface{}, 0, len(executedEvents)+1)
		args = append(args, instance.InstanceID)
		for _, e := range executedEvents {
			args = append(args, e.ID)
		}

		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf(`DELETE FROM pending_events WHERE instance_id = ? AND id IN (?%v)`, strings.Repeat(",?", len(executedEvents)-1)),
			args...,
		); err != nil {
			return errors.Wrap(err, "could not delete handled new events")
		}
	}

	// Add events from last execution to history
	if err := insertHistoryEvents(ctx, tx, instance.InstanceID, executedEvents); err != nil {
		return errors.Wrap(err, "could not insert new history events")
	}

	// Schedule activities
	for _, event := range activityEvents {
		if err := scheduleActivity(ctx, tx, instance.InstanceID, instance.ExecutionID, event); err != nil {
			return errors.Wrap(err, "could not schedule activity")
		}
	}

	// Insert new workflow events
	groupedEvents := make(map[*workflow.Instance][]history.Event)
	for _, m := range workflowEvents {
		if _, ok := groupedEvents[m.WorkflowInstance]; !ok {
			groupedEvents[m.WorkflowInstance] = []history.Event{}
		}

		groupedEvents[m.WorkflowInstance] = append(groupedEvents[m.WorkflowInstance], m.HistoryEvent)
	}

	for targetInstance, events := range groupedEvents {
		if instance.InstanceID != targetInstance.InstanceID {
			// Create new instance
			if err := createInstance(ctx, tx, targetInstance, true); err != nil {
				return err
			}
		}

		// Insert pending events for target instance
		if err := insertNewEvents(ctx, tx, targetInstance.InstanceID, events); err != nil {
			return errors.Wrap(err, "could not insert messages")
		}
	}

	return tx.Commit()
}

func (sb *sqliteBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *workflow.Instance) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(sb.options.WorkflowLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = ? WHERE id = ? AND execution_id = ? AND worker = ?`,
		until,
		instance.InstanceID,
		instance.ExecutionID,
		sb.workerName,
	)
	if err != nil {
		return errors.Wrap(err, "could not extend workflow task lock")
	}

	if rowsAffected, err := res.RowsAffected(); err != nil {
		return errors.Wrap(err, "could not determine if workflow task was extended")
	} else if rowsAffected == 0 {
		return errors.New("could not extend workflow task")
	}

	return tx.Commit()
}

func (sb *sqliteBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next activity
	// (work around missing LIMIT support in sqlite driver for UPDATE statements by using sub-query)
	now := time.Now()
	row, err := tx.QueryContext(
		ctx,
		`UPDATE activities
			SET locked_until = ?, worker = ?
			WHERE rowid = (
				SELECT rowid FROM activities WHERE locked_until IS NULL OR locked_until < ? LIMIT 1
			) RETURNING id, instance_id, execution_id, event_type, timestamp, schedule_event_id, attributes, visible_at`,
		now.Add(sb.options.ActivityLockTimeout),
		sb.workerName,
		now,
	)
	if err != nil {
		return nil, err
	}

	if !row.Next() {
		// No activity locked, abort
		return nil, nil
	}

	var instanceID, executionID string
	var attributes []byte
	event := history.Event{}

	if err := row.Scan(&event.ID, &instanceID, &executionID, &event.Type, &event.Timestamp, &event.ScheduleEventID, &attributes, &event.VisibleAt); err != nil {
		return nil, errors.Wrap(err, "could not scan event")
	}

	a, err := history.DeserializeAttributes(event.Type, attributes)
	if err != nil {
		return nil, errors.Wrap(err, "could not deserialize attributes")
	}

	event.Attributes = a

	t := &task.Activity{
		ID:               event.ID,
		WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
		Event:            event,
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

func (sb *sqliteBackend) CompleteActivityTask(ctx context.Context, instance *workflow.Instance, id string, event history.Event) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove activity
	if res, err := tx.ExecContext(
		ctx,
		`DELETE FROM activities WHERE instance_id = ? AND id = ? AND worker = ?`,
		instance.InstanceID,
		id,
		sb.workerName,
	); err != nil {
		return errors.Wrap(err, "could not unlock instance")
	} else if n, err := res.RowsAffected(); err != nil {
		return errors.Wrap(err, "could not check for deleted activities")
	} else if n != 1 {
		return errors.New("could not find activity to delete")
	}

	// Insert new event generated during this workflow execution
	if err := insertNewEvents(ctx, tx, instance.InstanceID, []history.Event{event}); err != nil {
		return errors.Wrap(err, "could not insert new events for completed activity")
	}

	return tx.Commit()
}

func (sb *sqliteBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(sb.options.ActivityLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = ? WHERE id = ? AND worker = ?`,
		until,
		activityID,
		sb.workerName,
	)
	if err != nil {
		return errors.Wrap(err, "could not extend activity lock")
	}

	if rowsAffected, err := res.RowsAffected(); err != nil {
		return errors.Wrap(err, "could not determine if activity was extended")
	} else if rowsAffected == 0 {
		return errors.New("could not extend activity")
	}

	return tx.Commit()
}
