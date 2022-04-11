package mysql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cschleiden/go-workflows/internal/history"
)

func insertNewEvents(ctx context.Context, tx *sql.Tx, instanceID string, newEvents []history.Event) error {
	return insertEvents(ctx, tx, "pending_events", instanceID, newEvents)
}

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instanceID string, historyEvents []history.Event) error {
	return insertEvents(ctx, tx, "history", instanceID, historyEvents)
}

func insertEvents(ctx context.Context, tx *sql.Tx, tableName string, instanceID string, events []history.Event) error {
	const batchSize = 20
	for batchStart := 0; batchStart < len(events); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(events) {
			batchEnd = len(events)
		}
		batchEvents := events[batchStart:batchEnd]

		query := "INSERT INTO `" + tableName + "` (event_id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)" +
			strings.Repeat(", (?, ?, ?, ?, ?, ?, ?, ?)", len(batchEvents)-1)

		args := make([]interface{}, 0, len(batchEvents)*7)

		for _, newEvent := range batchEvents {
			a, err := history.SerializeAttributes(newEvent.Attributes)
			if err != nil {
				return err
			}

			args = append(args, newEvent.ID, newEvent.SequenceID, instanceID, newEvent.Type, newEvent.Timestamp, newEvent.ScheduleEventID, a, newEvent.VisibleAt)
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
