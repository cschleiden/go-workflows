package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/pkg/errors"
)

func getPendingEvents(ctx context.Context, tx *sql.Tx, instanceID string) ([]history.Event, error) {
	now := time.Now()
	events, err := tx.QueryContext(ctx, "SELECT * FROM `pending_events` WHERE instance_id = ? AND (`visible_at` IS NULL OR `visible_at` <= ?)", instanceID, now)
	if err != nil {
		return nil, errors.Wrap(err, "could not get new events")
	}

	pendingEvents := make([]history.Event, 0)

	for events.Next() {
		pendingEvent, err := scanEvent(events)
		if err != nil {
			return nil, errors.Wrap(err, "could not read event")
		}

		pendingEvents = append(pendingEvents, pendingEvent)
	}

	return pendingEvents, nil
}

func getHistory(ctx context.Context, tx *sql.Tx, instanceID string) ([]history.Event, error) {
	historyEvents, err := tx.QueryContext(ctx, "SELECT * FROM `history` WHERE instance_id = ?", instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get history")
	}

	events := make([]history.Event, 0)

	for historyEvents.Next() {
		historyEvent, err := scanEvent(historyEvents)
		if err != nil {
			return nil, errors.Wrap(err, "could not read event")
		}

		events = append(events, historyEvent)
	}

	return events, nil
}

type Scanner interface {
	Scan(dest ...interface{}) error
}

func scanEvent(row Scanner) (history.Event, error) {
	var instanceID string
	var attributes []byte

	historyEvent := history.Event{}

	if err := row.Scan(&historyEvent.ID, &instanceID, &historyEvent.Type, &historyEvent.Timestamp, &historyEvent.ScheduleEventID, &attributes, &historyEvent.VisibleAt); err != nil {
		return historyEvent, errors.Wrap(err, "could not scan event")
	}

	a, err := history.DeserializeAttributes(historyEvent.Type, attributes)
	if err != nil {
		return historyEvent, errors.Wrap(err, "could not deserialize attributes")
	}

	historyEvent.Attributes = a

	return historyEvent, nil
}

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

		query := "INSERT INTO `" + tableName + "` (id, instance_id, event_type, timestamp, schedule_event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?)" +
			strings.Repeat(", (?, ?, ?, ?, ?, ?, ?)", len(batchEvents)-1)

		args := make([]interface{}, 0, len(batchEvents)*7)

		for _, newEvent := range batchEvents {
			a, err := history.SerializeAttributes(newEvent.Attributes)
			if err != nil {
				return err
			}

			args = append(args, newEvent.ID, instanceID, newEvent.Type, newEvent.Timestamp, newEvent.ScheduleEventID, a, newEvent.VisibleAt)
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
