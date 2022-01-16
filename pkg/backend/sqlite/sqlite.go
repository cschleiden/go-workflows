package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

var WorkflowLockTimeout = time.Minute * 1
var ActivityLockTimeout = time.Minute * 2

func NewSqliteBackend(path string) backend.Backend {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v", path))
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
	}
}

type sqliteBackend struct {
	db         *sql.DB
	workerName string
}

func (sb *sqliteBackend) CreateWorkflowInstance(ctx context.Context, m core.WorkflowEvent) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "could not start transaction")
	}
	defer tx.Rollback()

	// Create workflow instance
	if err := createInstance(ctx, tx, m.WorkflowInstance); err != nil {
		return err
	}

	// Initial history is empty, store only new events
	if err := insertNewEvents(ctx, tx, m.WorkflowInstance.GetInstanceID(), []history.Event{m.HistoryEvent}); err != nil {
		return errors.Wrap(err, "could not insert new event")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	return nil
}

func createInstance(ctx context.Context, tx *sql.Tx, wfi core.WorkflowInstance) error {
	var parentInstanceID *string
	var parentEventID *int
	if wfi.SubWorkflow() {
		i := wfi.ParentInstance().GetInstanceID()
		parentInstanceID = &i

		n := wfi.ParentInstance().ParentEventID()
		parentEventID = &n
	}

	if _, err := tx.ExecContext(
		ctx,
		"INSERT OR IGNORE INTO `instances` (id, execution_id, parent_instance_id, parent_event_id) VALUES (?, ?, ?, ?)",
		wfi.GetInstanceID(),
		wfi.GetExecutionID(),
		parentInstanceID,
		parentEventID,
	); err != nil {
		return errors.Wrap(err, "could not insert workflow instance")
	}

	return nil
}

func (sb *sqliteBackend) SignalWorkflow(ctx context.Context, instance core.WorkflowInstance, event history.Event) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), []history.Event{event}); err != nil {
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
	now := time.Now().UTC()
	row := tx.QueryRowContext(
		ctx,
		`UPDATE instances
			SET locked_until = ?, locked_by = ?
			WHERE rowid = (
				SELECT rowid FROM instances i
					WHERE (locked_until IS NULL OR locked_until < ?) AND completed_at IS NULL
						AND EXISTS (
							SELECT 1
								FROM pending_events
								WHERE instance_id = i.id AND execution_id = i.execution_id AND (visible_at IS NULL OR visible_at <= ?)
						)
					LIMIT 1
			) RETURNING id, execution_id, parent_instance_id, parent_event_id`,
		now.Add(WorkflowLockTimeout),
		sb.workerName,
		now,
		now,
	)

	var instanceID, executionID string
	var parentInstanceID *string
	var parentEventID *int
	if err := row.Scan(&instanceID, &executionID, &parentInstanceID, &parentEventID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	var wfi core.WorkflowInstance
	if parentInstanceID != nil {
		wfi = core.NewSubWorkflowInstance(instanceID, executionID, core.NewWorkflowInstance(*parentInstanceID, ""), *parentEventID)
	} else {
		wfi = core.NewWorkflowInstance(instanceID, executionID)
	}

	t := &task.Workflow{
		WorkflowInstance: wfi,
		NewEvents:        []history.Event{},
		History:          []history.Event{},
	}

	// Get new events
	events, err := tx.QueryContext(ctx, "SELECT * FROM `pending_events` WHERE instance_id = ? AND (`visible_at` IS NULL OR `visible_at` <= ?)", instanceID, now)
	if err != nil {
		return nil, errors.Wrap(err, "could not get new events")
	}

	for events.Next() {
		var instanceID string
		var attributes []byte

		historyEvent := history.Event{}

		if err := events.Scan(&historyEvent.ID, &instanceID, &historyEvent.EventType, &historyEvent.EventID, &attributes, &historyEvent.VisibleAt); err != nil {
			return nil, errors.Wrap(err, "could not scan event")
		}

		a, err := history.DeserializeAttributes(historyEvent.EventType, attributes)
		if err != nil {
			return nil, errors.Wrap(err, "could not deserialize attributes")
		}

		historyEvent.Attributes = a

		t.NewEvents = append(t.NewEvents, historyEvent)
	}

	// Return if there aren't any new events
	if len(t.NewEvents) == 0 {
		return nil, nil
	}

	// Get historyEvents
	historyEvents, err := tx.QueryContext(ctx, "SELECT * FROM `history` WHERE instance_id = ?", instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get history")
	}

	for historyEvents.Next() {
		var instanceID string
		var attributes []byte

		historyEvent := history.Event{}

		if err := historyEvents.Scan(&historyEvent.ID, &instanceID, &historyEvent.EventType, &historyEvent.EventID, &attributes, &historyEvent.VisibleAt); err != nil {
			return nil, errors.Wrap(err, "could not scan event")
		}

		a, err := history.DeserializeAttributes(historyEvent.EventType, attributes)
		if err != nil {
			return nil, errors.Wrap(err, "could not deserialize attributes")
		}

		historyEvent.Attributes = a

		t.History = append(t.History, historyEvent)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

func (sb *sqliteBackend) CompleteWorkflowTask(
	ctx context.Context,
	task task.Workflow,
	events []history.Event,
	workflowEvents []core.WorkflowEvent,
) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unlock instance
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = NULL, locked_by = NULL WHERE id = ? AND execution_id = ?`,
		task.WorkflowInstance.GetInstanceID(),
		task.WorkflowInstance.GetExecutionID(),
	); err != nil {
		return errors.Wrap(err, "could not unlock instance")
	}

	changedRows := 0
	if err := tx.QueryRowContext(ctx, "SELECT CHANGES()").Scan(&changedRows); err != nil {
		return errors.Wrap(err, "could not check for unlocked workflow instances")
	}

	if changedRows != 1 {
		return errors.New("could not find workflow instance to unlock")
	}

	// Remove handled events from task
	if len(task.NewEvents) > 0 {
		args := make([]interface{}, 0, len(task.NewEvents)+1)
		args = append(args, task.WorkflowInstance.GetInstanceID())
		for _, e := range task.NewEvents {
			args = append(args, e.ID)
		}

		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf(`DELETE FROM pending_events WHERE instance_id = ? AND id IN (?%v)`, strings.Repeat(",?", len(task.NewEvents)-1)),
			args...,
		); err != nil {
			return errors.Wrap(err, "could not delete handled new events")
		}
	}

	// Add all handled events to history
	if err := insertHistoryEvents(ctx, tx, task.WorkflowInstance.GetInstanceID(), task.NewEvents); err != nil {
		return errors.Wrap(err, "could not insert new events")
	}

	// Insert new events generated during this workflow execution to the history
	if err := insertHistoryEvents(ctx, tx, task.WorkflowInstance.GetInstanceID(), events); err != nil {
		return errors.Wrap(err, "could not insert new history events")
	}

	workflowCompleted := false

	// Schedule activities
	for _, e := range events {
		switch e.EventType {
		case history.EventType_ActivityScheduled:
			if err := scheduleActivity(ctx, tx, task.WorkflowInstance.GetInstanceID(), task.WorkflowInstance.GetExecutionID(), e); err != nil {
				return errors.Wrap(err, "could not schedule activity")
			}

		case history.EventType_WorkflowExecutionFinished:
			workflowCompleted = true
		}
	}

	// Insert new workflow events
	groupedEvents := make(map[core.WorkflowInstance][]history.Event)
	for _, m := range workflowEvents {
		if _, ok := groupedEvents[m.WorkflowInstance]; !ok {
			groupedEvents[m.WorkflowInstance] = []history.Event{}
		}

		groupedEvents[m.WorkflowInstance] = append(groupedEvents[m.WorkflowInstance], m.HistoryEvent)
	}

	for instance, events := range groupedEvents {
		if instance.GetInstanceID() != task.WorkflowInstance.GetInstanceID() {
			// Create new instance
			if err := createInstance(ctx, tx, instance); err != nil {
				return err
			}
		}

		if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), events); err != nil {
			return errors.Wrap(err, "could not insert messages")
		}
	}

	if workflowCompleted {
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE instances SET completed_at = ? WHERE id = ? AND execution_id = ?",
			time.Now().UTC(),
			task.WorkflowInstance.GetInstanceID(),
			task.WorkflowInstance.GetExecutionID(),
		); err != nil {
			return errors.Wrap(err, "could not mark instance as completed")
		}
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
	now := time.Now().UTC()
	row, err := tx.QueryContext(
		ctx,
		`UPDATE activities
			SET locked_until = ?, locked_by = ?
			WHERE rowid = (
				SELECT rowid FROM activities WHERE locked_until IS NULL OR locked_until < ? LIMIT 1
			) RETURNING id, instance_id, execution_id, event_type, event_id, attributes, visible_at`,
		now.Add(ActivityLockTimeout),
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

	if err := row.Scan(&event.ID, &instanceID, &executionID, &event.EventType, &event.EventID, &attributes, &event.VisibleAt); err != nil {
		return nil, errors.Wrap(err, "could not scan event")
	}

	a, err := history.DeserializeAttributes(event.EventType, attributes)
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

func (sb *sqliteBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, id string, event history.Event) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove activity
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM activities WHERE instance_id = ? AND id = ? AND locked_by = ?`,
		instance.GetInstanceID(),
		id,
		sb.workerName,
	); err != nil {
		return errors.Wrap(err, "could not unlock instance")
	}

	changedRows := 0
	if err := tx.QueryRowContext(ctx, "SELECT CHANGES()").Scan(&changedRows); err != nil {
		return errors.Wrap(err, "could not check for deleted activities")
	}

	if changedRows != 1 {
		return errors.New("could not find activity to delete")
	}

	// Insert new event generated during this workflow execution
	if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), []history.Event{event}); err != nil {
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

	until := time.Now().UTC().Add(ActivityLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = ? WHERE id = ? AND locked_by = ?`,
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

func insertNewEvents(ctx context.Context, tx *sql.Tx, instanceID string, newEvents []history.Event) error {
	for _, newEvent := range newEvents {
		a, err := history.SerializeAttributes(newEvent.Attributes)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `pending_events` (id, instance_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
			newEvent.ID,
			instanceID,
			newEvent.EventType,
			newEvent.EventID,
			a,
			newEvent.VisibleAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instanceID string, historyEvents []history.Event) error {
	for _, historyEvent := range historyEvents {
		a, err := history.SerializeAttributes(historyEvent.Attributes)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `history` (id, instance_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
			historyEvent.ID,
			instanceID,
			historyEvent.EventType,
			historyEvent.EventID,
			a,
			historyEvent.VisibleAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func scheduleActivity(ctx context.Context, tx *sql.Tx, instanceID, executionID string, event history.Event) error {
	a, err := history.SerializeAttributes(event.Attributes)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO activities
			(id, instance_id, execution_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		instanceID,
		executionID,
		event.EventType,
		event.EventID,
		a,
		event.VisibleAt,
	)

	return err
}
