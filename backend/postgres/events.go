package postgres

import (
	"context"
	"database/sql"

	"github.com/cschleiden/go-workflows/internal/history"
)

func insertPendingEvents(ctx context.Context, tx *sql.Tx, instanceID string, newEvents []history.Event) error {
	return insertEvents(ctx, tx, "pending_events", instanceID, newEvents)
}

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instanceID string, historyEvents []history.Event) error {
	return insertEvents(ctx, tx, "history", instanceID, historyEvents)
}

func insertEvents(ctx context.Context, tx *sql.Tx, tableName string, instanceID string, events []history.Event) error {
	for _, newEvent := range events {
		a, err := history.SerializeAttributes(newEvent.Attributes)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(
			ctx,
			"INSERT INTO gwf."+tableName+" (event_id, sequence_id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
			newEvent.ID,
			newEvent.SequenceID,
			instanceID,
			newEvent.Type,
			newEvent.Timestamp,
			newEvent.ScheduleEventID,
			string(a),
			newEvent.VisibleAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func removeFutureEvent(ctx context.Context, tx *sql.Tx, instanceID string, scheduleEventID int64) error {
	_, err := tx.ExecContext(
		ctx,
		"DELETE FROM gwf.pending_events WHERE instance_id = $1 AND schedule_event_id = $2 AND visible_at IS NOT NULL",
		instanceID,
		scheduleEventID,
	)

	return err
}
