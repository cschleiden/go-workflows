package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func addPendingEventToStreamP(ctx context.Context, p redis.Pipeliner, instanceID string, event *history.Event) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	// Add event reference
	streamKey := pendingEventsKey(instanceID)
	if err := p.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"event": event.ID,
		},
	}).Err(); err != nil {
		return fmt.Errorf("adding event to stream: %w", err)
	}

	// Add event data
	if err := p.Set(ctx, eventKey(event.ID), eventData, 0).Err(); err != nil {
		return fmt.Errorf("setting event payload: %w", err)
	}

	return nil
}

// addEventsToStream adds the given events to the given event stream. If successful, the message id of the last event added
// is returned
// KEYS[1] - stream key
// KEYS[1 + i] - event keys
// ARGV[i] - event data as serialized strings
//   - [i + 0] - history id
//   - [i + 1] - event id
//   - [i + 2] - event data
var addEventsToHistoryStreamCmd = redis.NewScript(`
	local msgID = ""
	local keyI = 2
	for i = 1, #ARGV, 3 do
		-- Store event reference in stream
		msgID = redis.call("XADD", KEYS[1], ARGV[i], "event", ARGV[i + 1])

		-- Store event
		redis.call("SET", KEYS[keyI], ARGV[i + 2])

		keyI = keyI + 1
	end
	return msgID
`)

func addEventsToHistoryStreamP(ctx context.Context, p redis.Pipeliner, streamKey string, events []history.Event) error {
	keys := make([]string, 0)
	keys = append(keys, streamKey)

	eventsData := make([]string, 0)
	for _, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshaling event: %w", err)
		}

		// log.Println("addEventsToHistoryStreamP:", event.SequenceID, string(eventData))

		keys = append(keys, eventKey(event.ID))
		eventsData = append(eventsData, historyID(event.SequenceID), event.ID, string(eventData))
	}

	return addEventsToHistoryStreamCmd.Run(ctx, p, keys, eventsData).Err()
}

// KEYS[1] - future event zset key
// KEYS[2] - future event key
// KEYS[3] - event key
// ARGV[1] - timestamp
// ARGV[2] - Instance ID
// ARGV[3] - event id
// ARGV[4] - event data
var addFutureEventCmd = redis.NewScript(`
	redis.call("ZADD", KEYS[1], ARGV[1], KEYS[2])
	redis.call("HSET", KEYS[2], "instance", ARGV[2], "event", ARGV[3])
	return redis.call("SET", KEYS[3], ARGV[4])
`)

func addFutureEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	addFutureEventCmd.Run(
		ctx, p,
		[]string{futureEventsKey(), futureEventKey(instance.InstanceID, event.ScheduleEventID), eventKey(event.ID)},
		strconv.FormatInt(event.VisibleAt.UnixMilli(), 10),
		instance.InstanceID,
		event.ID,
		string(eventData),
	)

	return nil
}

// KEYS[1] - future event zset key
// KEYS[2] - future event key
var removeFutureEventCmd = redis.NewScript(`
	redis.call("ZREM", KEYS[1], KEYS[2])
	return redis.call("DEL", KEYS[2])
`)

// removeFutureEvent removes a scheduled future event for the given event. Events are associated via their ScheduleEventID
func removeFutureEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) {
	removeFutureEventCmd.Run(
		ctx,
		p,
		[]string{futureEventsKey(), futureEventKey(instance.InstanceID, event.ScheduleEventID)})
}

func fetchStreamEvents(ctx context.Context, rdb redis.UniversalClient, msgs []redis.XMessage, historyEvents bool) ([]history.Event, error) {
	eventKeys := make([]string, 0)
	for _, msg := range msgs {
		eventID := msg.Values["event"].(string)
		eventKeys = append(eventKeys, eventKey(eventID))
	}

	// Fetch events
	eventData, err := rdb.MGet(ctx, eventKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("getting events: %w", err)
	}

	events := make([]history.Event, 0)
	for i, data := range eventData {
		var event history.Event
		if err := json.Unmarshal([]byte(data.(string)), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		if historyEvents {
			// Restore sequence id
			sequenceID, err := sequenceIdFromHistoryID(msgs[i].ID)
			if err != nil {
				return nil, fmt.Errorf("getting sequence id: %w", err)
			}
			event.SequenceID = sequenceID
		}

		events = append(events, event)
	}

	return events, nil
}
