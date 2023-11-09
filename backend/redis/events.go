package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"github.com/redis/go-redis/v9"
)

type eventWithoutAttributes struct {
	*history.Event
}

func (e *eventWithoutAttributes) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		*history.Event
		Attributes interface{} `json:"attr"`
	}{
		Event:      e.Event,
		Attributes: nil,
	})
}

func marshalEventWithoutAttributes(event *history.Event) (string, error) {
	data, err := json.Marshal(&eventWithoutAttributes{event})
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// KEYS[1..n] - payload keys
// ARGV[1..n] - payload values
var addPayloadsCmd = redis.NewScript(`
	for i = 1, #ARGV do
		redis.pcall("SET", KEYS[i], ARGV[i], "NX")
	end

	return 0
`)

func addEventPayloads(ctx context.Context, p redis.Pipeliner, events []*history.Event) error {
	keys := make([]string, 0)
	values := make([]interface{}, 0)

	for _, event := range events {
		payload, err := json.Marshal(event.Attributes)
		if err != nil {
			return fmt.Errorf("marshaling event payload: %w", err)
		}

		keys = append(keys, payloadKey(event.ID))
		values = append(values, string(payload))
	}

	return addPayloadsCmd.Run(ctx, p, keys, values...).Err()
}

func addEventToStreamP(ctx context.Context, p redis.Pipeliner, streamKey string, event *history.Event) error {
	eventData, err := marshalEventWithoutAttributes(event)
	if err != nil {
		return err
	}

	return p.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	}).Err()
}

// addEventsToStream adds the given events to the given event stream. If successful, the message id of the last event added
// is returned
// KEYS[1] - stream key
// ARGV[1] - event data as serialized strings
var addEventsToStreamCmd = redis.NewScript(`
	local msgID = ""
	for i = 1, #ARGV, 2 do
		msgID = redis.call("XADD", KEYS[1], ARGV[i], "event", ARGV[i + 1])
	end
	return msgID
`)

func addEventsToStreamP(ctx context.Context, p redis.Pipeliner, streamKey string, events []*history.Event) error {
	eventsData := make([]string, 0)
	for _, event := range events {
		eventData, err := marshalEventWithoutAttributes(event)
		if err != nil {
			return err
		}

		// log.Println("addEventsToHistoryStreamP:", event.SequenceID, string(eventData))

		eventsData = append(eventsData, historyID(event.SequenceID))
		eventsData = append(eventsData, string(eventData))
	}

	addEventsToStreamCmd.Run(ctx, p, []string{streamKey}, eventsData)

	return nil
}

// Adds an event to be delivered in the future. Not cluster-safe.
// KEYS[1] - future event zset key
// KEYS[2] - future event key
// KEYS[3] - future event payload key
// ARGV[1] - timestamp
// ARGV[2] - Instance segment
// ARGV[3] - event data
// ARGV[4] - event payload
var addFutureEventCmd = redis.NewScript(`
	redis.call("ZADD", KEYS[1], ARGV[1], KEYS[2])
	redis.call("HSET", KEYS[2], "instance", ARGV[2], "event", ARGV[3], "payload", KEYS[3])
	redis.call("SET", KEYS[3], ARGV[4], "NX")
	return 0
`)

func addFutureEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) error {
	eventData, err := marshalEventWithoutAttributes(event)
	if err != nil {
		return err
	}

	payloadEventData, err := json.Marshal(event.Attributes)
	if err != nil {
		return err
	}

	return addFutureEventCmd.Run(
		ctx, p,
		[]string{futureEventsKey(), futureEventKey(instance, event.ScheduleEventID), payloadKey(event.ID)},
		strconv.FormatInt(event.VisibleAt.UnixMilli(), 10),
		instanceSegment(instance),
		string(eventData),
		string(payloadEventData),
	).Err()
}

// Remove a scheduled future event. Not cluster-safe.
// KEYS[1] - future event zset key
// KEYS[2] - future event key
var removeFutureEventCmd = redis.NewScript(`
	redis.call("ZREM", KEYS[1], KEYS[2])
	local k = redis.call("HGET", KEYS[2], "payload")
	redis.call("DEL", k)
	return redis.call("DEL", KEYS[2])
`)

// removeFutureEvent removes a scheduled future event for the given event. Events are associated via their ScheduleEventID
func removeFutureEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) {
	key := futureEventKey(instance, event.ScheduleEventID)
	removeFutureEventCmd.Run(ctx, p, []string{futureEventsKey(), key})
}
