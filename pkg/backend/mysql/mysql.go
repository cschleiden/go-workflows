package mysql

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
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

//go:embed schema.sql
var schema string

var WorkflowLockTimeout = time.Minute * 1
var ActivityLockTimeout = time.Minute * 2

func NewMysqlBackend(user, password, database string) backend.Backend {
	dsn := fmt.Sprintf("%s:%s@/%s?parseTime=true&interpolateParams=true", user, password, database)

	schemaDsn := dsn + "&multiStatements=true"
	db, err := sql.Open("mysql", schemaDsn)
	if err != nil {
		panic(err)
	}

	if _, err := db.Exec(schema); err != nil {
		panic(errors.Wrap(err, "could not initialize database"))
	}

	if err := db.Close(); err != nil {
		panic(err)
	}

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}

	return &mysqlBackend{
		db:         db,
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
	}
}

type mysqlBackend struct {
	db         *sql.DB
	workerName string
}

// CreateWorkflowInstance creates a new workflow instance
func (b *mysqlBackend) CreateWorkflowInstance(ctx context.Context, m core.WorkflowEvent) error {
	tx, err := b.db.BeginTx(ctx, nil)
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
		"INSERT IGNORE INTO `instances` (instance_id, execution_id, parent_instance_id, parent_event_id) VALUES (?, ?, ?, ?)",
		wfi.GetInstanceID(),
		wfi.GetExecutionID(),
		parentInstanceID,
		parentEventID,
	); err != nil {
		return errors.Wrap(err, "could not insert workflow instance")
	}

	return nil
}

func insertNewEvents(ctx context.Context, tx *sql.Tx, instanceID string, newEvents []history.Event) error {
	for _, newEvent := range newEvents {
		a, err := history.SerializeAttributes(newEvent.Attributes)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `pending_events` (event_id, instance_id, event_type, event_id2, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
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

// SignalWorkflow signals a running workflow instance
func (b *mysqlBackend) SignalWorkflow(ctx context.Context, instance core.WorkflowInstance, event history.Event) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), []history.Event{event}); err != nil {
		return errors.Wrap(err, "could not insert signal event")
	}

	return tx.Commit()
}

// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
func (b *mysqlBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next workflow task by finding an unlocked instance with new events to process.
	now := time.Now().UTC()
	row := tx.QueryRowContext(
		ctx,
		`SELECT i.id, i.instance_id, i.execution_id, i.parent_instance_id, i.parent_event_id FROM instances i
			INNER JOIN pending_events pe ON i.instance_id = pe.instance_id
			WHERE (i.locked_until IS NULL OR i.locked_until < ?) AND i.completed_at IS NULL
				AND (pe.visible_at IS NULL OR pe.visible_at <= ?)
			LIMIT 1
			FOR UPDATE SKIP LOCKED`,
		now,
		now,
	)

	var id int
	var instanceID, executionID string
	var parentInstanceID *string
	var parentEventID *int
	if err := row.Scan(&id, &instanceID, &executionID, &parentInstanceID, &parentEventID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, errors.Wrap(err, "could not scan workflow instance id")
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances i
			SET locked_until = ?, locked_by = ?
			WHERE id = ?`,
		now.Add(WorkflowLockTimeout),
		b.workerName,
		id,
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not lock workflow instance")
	}

	if affectedRows, err := res.RowsAffected(); err != nil {
		return nil, errors.Wrap(err, "could not lock workflow instance")
	} else if affectedRows == 0 {
		// No instance locked?
		return nil, nil
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
	events, err := tx.QueryContext(
		ctx,
		"SELECT event_id, instance_id, event_type, event_id2, attributes, visible_at FROM `pending_events` WHERE instance_id = ? AND (`visible_at` IS NULL OR `visible_at` <= ?) ORDER BY id",
		instanceID,
		now,
	)
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
	historyEvents, err := tx.QueryContext(
		ctx,
		"SELECT event_id, instance_id, event_type, event_id2, attributes, visible_at FROM `history` WHERE instance_id = ? ORDER BY id",
		instanceID,
	)
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

// CompleteWorkflowTask completes a workflow task retrieved using GetWorkflowTask
//
// This checkpoints the execution. events are new events from the last workflow execution
// which will be added to the workflow instance history. workflowEvents are new events for the
// completed or other workflow instances.
func (b *mysqlBackend) CompleteWorkflowTask(ctx context.Context, task task.Workflow, events []history.Event, workflowEvents []core.WorkflowEvent) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unlock instance
	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = NULL, locked_by = NULL WHERE instance_id = ? AND execution_id = ?`,
		task.WorkflowInstance.GetInstanceID(),
		task.WorkflowInstance.GetExecutionID(),
	)
	if err != nil {
		return errors.Wrap(err, "could not unlock instance")
	}

	changedRows, err := res.RowsAffected()
	if err != nil {
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
			fmt.Sprintf(`DELETE FROM pending_events WHERE instance_id = ? AND event_id IN (?%v)`, strings.Repeat(",?", len(task.NewEvents)-1)),
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
			"UPDATE instances SET completed_at = ? WHERE instance_id = ? AND execution_id = ?",
			time.Now().UTC(),
			task.WorkflowInstance.GetInstanceID(),
			task.WorkflowInstance.GetExecutionID(),
		); err != nil {
			return errors.Wrap(err, "could not mark instance as completed")
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (sb *mysqlBackend) ExtendWorkflowTask(ctx context.Context, instance core.WorkflowInstance) error {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().UTC().Add(WorkflowLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = ? WHERE instance_id = ? AND execution_id = ? AND locked_by = ?`,
		until,
		instance.GetInstanceID(),
		instance.GetExecutionID(),
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

// GetActivityTask returns a pending activity task or nil if there are no pending activities
func (b *mysqlBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next activity
	now := time.Now().UTC()
	res := tx.QueryRowContext(
		ctx,
		`SELECT id, activity_id, instance_id, execution_id, event_type, event_id, attributes, visible_at
			FROM activities
			WHERE locked_until IS NULL OR locked_until < ?
			LIMIT 1
			FOR UPDATE SKIP LOCKED`,
		now,
	)

	var id int
	var instanceID, executionID string
	var attributes []byte
	event := history.Event{}

	if err := res.Scan(&id, &event.ID, &instanceID, &executionID, &event.EventType, &event.EventID, &attributes, &event.VisibleAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, errors.Wrap(err, "could not find activity task to lock")
	}

	a, err := history.DeserializeAttributes(event.EventType, attributes)
	if err != nil {
		return nil, errors.Wrap(err, "could not deserialize attributes")
	}

	event.Attributes = a

	if _, err := tx.ExecContext(ctx, `UPDATE activities SET locked_until = ?, locked_by = ? WHERE id = ?`, now.Add(ActivityLockTimeout), b.workerName, id); err != nil {
		return nil, errors.Wrap(err, "could not lock activity")
	}

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

// CompleteActivityTask completes a activity task retrieved using GetActivityTask
func (b *mysqlBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, id string, event history.Event) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove activity
	if res, err := tx.ExecContext(
		ctx,
		`DELETE FROM activities WHERE activity_id = ? AND instance_id = ? AND execution_id = ? AND locked_by = ?`,
		id,
		instance.GetInstanceID(),
		instance.GetExecutionID(),
		b.workerName,
	); err != nil {
		return errors.Wrap(err, "could not complete activity")
	} else {
		affected, err := res.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "could not check for completed activity")
		}

		if affected == 0 {
			return errors.New("could not find locked activity")
		}
	}

	// Insert new event generated during this workflow execution
	if err := insertNewEvents(ctx, tx, instance.GetInstanceID(), []history.Event{event}); err != nil {
		return errors.Wrap(err, "could not insert new events for completed activity")
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (b *mysqlBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().UTC().Add(ActivityLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = ? WHERE activity_id = ? AND locked_by = ?`,
		until,
		activityID,
		b.workerName,
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

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instanceID string, historyEvents []history.Event) error {
	for _, historyEvent := range historyEvents {
		a, err := history.SerializeAttributes(historyEvent.Attributes)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `history` (event_id, instance_id, event_type, event_id2, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
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
			(activity_id, instance_id, execution_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
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