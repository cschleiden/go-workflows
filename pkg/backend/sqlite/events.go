package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/pkg/errors"
)

func getPendingEvents(ctx context.Context, tx *sql.Tx, instanceID string) ([]history.Event, error) {
	now := time.Now().UTC()
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

	if err := row.Scan(&historyEvent.ID, &instanceID, &historyEvent.Type, &historyEvent.Timestamp, &historyEvent.EventID, &attributes, &historyEvent.VisibleAt); err != nil {
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
	for _, newEvent := range newEvents {
		a, err := history.SerializeAttributes(newEvent.Attributes)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO `pending_events` (id, instance_id, event_type, timestamp, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			newEvent.ID,
			instanceID,
			newEvent.Type,
			newEvent.Timestamp,
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
			"INSERT INTO `history` (id, instance_id, event_type, timestamp, event_id, attributes, visible_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			historyEvent.ID,
			instanceID,
			historyEvent.Type,
			historyEvent.Timestamp,
			historyEvent.EventID,
			a,
			historyEvent.VisibleAt,
		); err != nil {
			return err
		}
	}

	return nil
}
