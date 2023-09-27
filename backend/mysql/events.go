package mysql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
)

func insertPendingEvents(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, newEvents []*history.Event) error {
	return insertEvents(ctx, tx, "pending_events", instance, newEvents)
}

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, historyEvents []*history.Event) error {
	return insertEvents(ctx, tx, "history", instance, historyEvents)
}

func insertEvents(ctx context.Context, tx *sql.Tx, tableName string, instance *core.WorkflowInstance, events []*history.Event) error {
	const batchSize = 20
	for batchStart := 0; batchStart < len(events); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(events) {
			batchEnd = len(events)
		}
		batchEvents := events[batchStart:batchEnd]

		query := "INSERT INTO `" + tableName +
			"` (event_id, sequence_id, instance_id, execution_id, event_type, timestamp, schedule_event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)" +
			strings.Repeat(", (?, ?, ?, ?, ?, ?, ?, ?, ?)", len(batchEvents)-1)

		args := make([]interface{}, 0, len(batchEvents)*7)

		for _, newEvent := range batchEvents {
			a, err := history.SerializeAttributes(newEvent.Attributes)
			if err != nil {
				return err
			}

			args = append(
				args,
				newEvent.ID, newEvent.SequenceID, instance.InstanceID, instance.ExecutionID, newEvent.Type, newEvent.Timestamp, newEvent.ScheduleEventID, a, newEvent.VisibleAt)
		}

		_, err := tx.ExecContext(
			ctx,
			query,
			args...,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func removeFutureEvent(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, scheduleEventID int64) error {
	_, err := tx.ExecContext(
		ctx,
		"DELETE FROM `pending_events` WHERE instance_id = ? AND execution_id = ? AND schedule_event_id = ? AND visible_at IS NOT NULL",
		instance.InstanceID,
		instance.ExecutionID,
		scheduleEventID,
	)

	return err
}
