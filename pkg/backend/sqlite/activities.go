package sqlite

import (
	"context"
	"database/sql"

	"github.com/cschleiden/go-workflows/pkg/history"
)

func scheduleActivity(ctx context.Context, tx *sql.Tx, instanceID, executionID string, event history.Event) error {
	attributes, err := history.SerializeAttributes(event.Attributes)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO activities
			(id, instance_id, execution_id, event_type, timestamp, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		instanceID,
		executionID,
		event.Type,
		event.Timestamp,
		event.EventID,
		attributes,
		event.VisibleAt,
	)

	return err
}
