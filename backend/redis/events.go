package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
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

// KEYS[1 - payload key
// ARGV[1..n] - payload values
var addPayloadsCmd = redis.NewScript(`
	for i = 1, #ARGV, 2 do
		redis.pcall("HSETNX", KEYS[1], ARGV[i], ARGV[i+1])
	end

	return 0
`)

func (rb *redisBackend) addEventPayloadsP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, events []*history.Event) error {
	args := make([]interface{}, 0)

	for _, event := range events {
		payload, err := json.Marshal(event.Attributes)
		if err != nil {
			return fmt.Errorf("marshaling event payload: %w", err)
		}

		args = append(args, event.ID, string(payload))
	}

	return addPayloadsCmd.Run(ctx, p, []string{rb.keys.payloadKey(instance)}, args...).Err()
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
