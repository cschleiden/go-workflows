package sqlite

import (
	"context"
	"database/sql"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
)

func scheduleActivity(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, event *history.Event) error {
	attributes, err := history.SerializeAttributes(event.Attributes)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO activities
			(id, instance_id, execution_id, event_type, timestamp, schedule_event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		instance.InstanceID,
		instance.ExecutionID,
		event.Type,
		event.Timestamp,
		event.ScheduleEventID,
		attributes,
		event.VisibleAt,
	)

	return err
}
