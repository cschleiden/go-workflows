package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
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
		db: db,
	}
}

type sqliteBackend struct {
	db *sql.DB
}

func (sb *sqliteBackend) CreateWorkflowInstance(ctx context.Context, m core.TaskMessage) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "could not start transaction")
	}
	defer tx.Rollback()

	// Create workflow instance
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO `instances` (id, execution_id) VALUES (?, ?)",
		m.WorkflowInstance.GetInstanceID(),
		m.WorkflowInstance.GetExecutionID(),
	); err != nil {
		return errors.Wrap(err, "could not insert workflow instance")
	}

	// Initial history is empty, store only new events
	if err := insertNewEvents(ctx, tx, m.WorkflowInstance.GetInstanceID(), []history.HistoryEvent{m.HistoryEvent}); err != nil {
		return errors.Wrap(err, "could not insert new event")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	return nil
}

func (sb *sqliteBackend) SignalWorkflow(ctx context.Context, instance core.WorkflowInstance, event history.HistoryEvent) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), []history.HistoryEvent{event}); err != nil {
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

	// Lock next workflow task.
	// (work around missing LIMIT support in sqlite driver for UPDATE statements by using sub-query)
	now := time.Now().UTC()
	row, err := tx.QueryContext(
		ctx,
		`UPDATE instances
			SET locked_until = ?, locked_by = ?
			WHERE rowid = (
				SELECT rowid FROM instances
					WHERE (locked_until IS NULL OR locked_until < ?) AND completed_at IS NULL
					LIMIT 1
			) RETURNING id, execution_id`,
		now.Add(WorkflowLockTimeout),
		"worker-id", // TODO: What to use for `locked_by`?
		now,
	)
	if err != nil {
		return nil, err
	}

	if !row.Next() {
		// No instance locked
		return nil, nil
	}

	var instanceID, executionID string
	if err := row.Scan(&instanceID, &executionID); err != nil {
		return nil, err
	}

	t := &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
		NewEvents:        []history.HistoryEvent{},
		History:          []history.HistoryEvent{},
	}

	// Get new events
	events, err := tx.QueryContext(ctx, "SELECT * FROM `new_events` WHERE instance_id = ? AND (`visible_at` IS NULL OR `visible_at` <= ?)", instanceID, now)
	if err != nil {
		return nil, errors.Wrap(err, "could not get new events")
	}

	for events.Next() {
		var instanceID, attributes string

		historyEvent := history.HistoryEvent{}

		if err := events.Scan(&historyEvent.ID, &instanceID, &historyEvent.EventType, &historyEvent.EventID, &attributes, &historyEvent.VisibleAt); err != nil {
			return nil, errors.Wrap(err, "could not scan event")
		}

		a, err := deserializeAttributes(historyEvent.EventType, attributes)
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
		var instanceID, attributes string

		historyEvent := history.HistoryEvent{}

		if err := historyEvents.Scan(&historyEvent.ID, &instanceID, &historyEvent.EventType, &historyEvent.EventID, &attributes, &historyEvent.VisibleAt); err != nil {
			return nil, errors.Wrap(err, "could not scan event")
		}

		a, err := deserializeAttributes(historyEvent.EventType, attributes)
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

func (sb *sqliteBackend) CompleteWorkflowTask(ctx context.Context, task task.Workflow, newEvents []history.HistoryEvent) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unlock instance
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = NULL, locked_by = NULL WHERE id = ?`,
		task.WorkflowInstance.GetInstanceID(),
	); err != nil {
		return errors.Wrap(err, "could not unlock instance")
	}

	// Remove handled events
	if len(task.NewEvents) > 0 {
		args := make([]interface{}, 0, len(task.NewEvents)+1)
		args = append(args, task.WorkflowInstance.GetInstanceID())
		for _, e := range task.NewEvents {
			args = append(args, e.ID)
		}

		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf(`DELETE FROM new_events WHERE instance_id = ? AND id IN (?%v)`, strings.Repeat(",?", len(task.NewEvents)-1)),
			args...,
		); err != nil {
			return errors.Wrap(err, "could not delete handled new events")
		}
	}

	// Insert new events generated during this workflow execution
	if err := insertNewEvents(ctx, tx, task.WorkflowInstance.GetInstanceID(), newEvents); err != nil {
		return errors.Wrap(err, "could not insert new events")
	}

	// TODO: Handle workflow finished?

	workflowCompleted := false

	// Schedule activities
	for _, e := range newEvents {
		switch e.EventType {
		case history.HistoryEventType_ActivityScheduled:
			if err := scheduleActivity(ctx, tx, task.WorkflowInstance.GetInstanceID(), e); err != nil {
				return errors.Wrap(err, "could not schedule activity")
			}
		case history.HistoryEventType_WorkflowExecutionFinished:
			workflowCompleted = true
		}
	}

	// Add all handled events to history
	if err := insertHistoryEvents(ctx, tx, task.WorkflowInstance.GetInstanceID(), task.NewEvents); err != nil {
		return errors.Wrap(err, "could not insert new events")
	}

	if workflowCompleted {
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE instances SET completed_at = ? WHERE id = ?", time.Now().UTC(), task.WorkflowInstance.GetInstanceID(),
		); err != nil {
			return errors.Wrap(err, "could not mark instance as completed")
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
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
			) RETURNING id, instance_id, event_type, event_id, attributes, visible_at`,
		now.Add(ActivityLockTimeout),
		"activity-worker-id", // TODO: What to use for `locked_by`?
		now,
	)
	if err != nil {
		return nil, err
	}

	if !row.Next() {
		// No activity locked, abort
		return nil, nil
	}

	var instanceID, attributes string
	event := history.HistoryEvent{}

	if err := row.Scan(&event.ID, &instanceID, &event.EventType, &event.EventID, &attributes, &event.VisibleAt); err != nil {
		return nil, errors.Wrap(err, "could not scan event")
	}

	a, err := deserializeAttributes(event.EventType, attributes)
	if err != nil {
		return nil, errors.Wrap(err, "could not deserialize attributes")
	}

	event.Attributes = a

	t := &task.Activity{
		ID:               event.ID,
		WorkflowInstance: core.NewWorkflowInstance(instanceID, ""), // TODO: Execution id
		Event:            event,
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

func (sb *sqliteBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, id string, event history.HistoryEvent) error {
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
		"activity-worker-id", // TODO: What to use for `locked_by`?
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
	if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), []history.HistoryEvent{event}); err != nil {
		return errors.Wrap(err, "could not insert new events for completed activity")
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func serializeAttributes(attributes interface{}) string {
	b, err := json.Marshal(attributes)
	if err != nil {
		panic(err)
	}

	return string(b)
}

func deserializeAttributes(eventType history.HistoryEventType, attributes string) (attr interface{}, err error) {
	switch eventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		attr = &history.ExecutionStartedAttributes{}
	case history.HistoryEventType_WorkflowExecutionFinished:
		attr = &history.ExecutionCompletedAttributes{}

	case history.HistoryEventType_ActivityScheduled:
		attr = &history.ActivityScheduledAttributes{}
	case history.HistoryEventType_ActivityCompleted:
		attr = &history.ActivityCompletedAttributes{}

	case history.HistoryEventType_SignalReceived:
		attr = &history.SignalReceivedAttributes{}

	case history.HistoryEventType_TimerScheduled:
		attr = &history.TimerScheduledAttributes{}
	case history.HistoryEventType_TimerFired:
		attr = &history.TimerFiredAttributes{}

	default:
		panic("unknown event type when deserializing attributes")
	}

	err = json.Unmarshal([]byte(attributes), &attr)
	return attr, err
}

func insertNewEvents(ctx context.Context, tx *sql.Tx, instanceID string, newEvents []history.HistoryEvent) error {
	for _, newEvent := range newEvents {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `new_events` (id, instance_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
			newEvent.ID,
			instanceID,
			newEvent.EventType,
			newEvent.EventID,
			serializeAttributes(newEvent.Attributes),
			newEvent.VisibleAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instanceID string, historyEvents []history.HistoryEvent) error {
	for _, historyEvent := range historyEvents {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `history` (id, instance_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
			historyEvent.ID,
			instanceID,
			historyEvent.EventType,
			historyEvent.EventID,
			serializeAttributes(historyEvent.Attributes),
			historyEvent.VisibleAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func scheduleActivity(ctx context.Context, tx *sql.Tx, instanceID string, event history.HistoryEvent) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO activities
			(id, instance_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)`,
		event.ID,
		instanceID,
		event.EventType,
		event.EventID,
		serializeAttributes(event.Attributes),
		event.VisibleAt,
	)

	return err
}
