package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/pkg/errors"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

func NewSqliteBackend(path string) backend.Backend {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=memory", path))
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

	// Initial history is empty, store only new event
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO `new_events` (id, instance_id, event_type, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?)",
		m.HistoryEvent.ID,
		m.WorkflowInstance.GetInstanceID(),
		m.HistoryEvent.EventType,
		m.HistoryEvent.EventID,
		serializeAttributes(m.HistoryEvent.Attributes),
		m.HistoryEvent.VisibleAt,
	); err != nil {
		return errors.Wrap(err, "could not insert new event")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	return nil
}

func (sb *sqliteBackend) SignalWorkflow(_ context.Context, _ core.WorkflowInstance, _ history.HistoryEvent) error {
	panic("not implemented") // TODO: Implement
}

func (sb *sqliteBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Work around missing LIMIT support in sqlite driver for UPDATE statements
	row, err := tx.QueryContext(
		ctx,
		"UPDATE `instances` SET locked_at = CURRENT_TIMESTAMP, locked_by = ? WHERE rowid = (SELECT rowid FROM `instances` WHERE locked_at IS NULL LIMIT 1) RETURNING id, execution_id",
		"test",
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
	events, err := tx.QueryContext(ctx, "SELECT * FROM `new_events` WHERE instance_id = ? AND (`visible_at` IS NULL OR `visible_at` < CURRENT_TIMESTAMP)", instanceID)
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

	// Get historyEvents
	historyEvents, err := tx.QueryContext(ctx, "SELECT * FROM `history` WHERE instance_id = ?", instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get history")
	}

	for historyEvents.Next() {
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

		t.History = append(t.History, historyEvent)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

func (sb *sqliteBackend) CompleteWorkflowTask(ctx context.Context, task task.Workflow, newEvents []history.HistoryEvent) error {
	panic("not implemented") // TODO: Implement
}

func (sb *sqliteBackend) GetActivityTask(_ context.Context) (*task.Activity, error) {
	return nil, nil
}

func (sb *sqliteBackend) CompleteActivityTask(_ context.Context, _ core.WorkflowInstance, _ string, _ history.HistoryEvent) error {
	panic("not implemented") // TODO: Implement
}

func serializeAttributes(attributes interface{}) string {
	b, err := json.Marshal(attributes)
	if err != nil {
		panic(err)
	}

	return string(b)
}

func deserializeAttributes(eventType history.HistoryEventType, attributes string) (interface{}, error) {
	switch eventType {
	case history.HistoryEventType_WorkflowExecutionStarted:
		var a history.ExecutionStartedAttributes
		if err := json.Unmarshal([]byte(attributes), &a); err != nil {
			return nil, err
		}

		return a, nil
	}

	panic("unknown event type")
}
