package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	redis "github.com/redis/go-redis/v9"
)

// Adds an event to be delivered in the future. Not cluster-safe.
// KEYS[1] - future event zset key
// KEYS[2] - future event key
// KEYS[3] - instance payload key
// ARGV[1] - timestamp/score for set
// ARGV[2] - Instance segment
// ARGV[3] - event id
// ARGV[4] - event data
// ARGV[5] - event payload
var addFutureEventCmd = redis.NewScript(`
	redis.call("ZADD", KEYS[1], ARGV[1], KEYS[2])
	redis.call("HSET", KEYS[2], "instance", ARGV[2], "id", ARGV[3], "event", ARGV[4])
	redis.call("HSETNX", KEYS[3], ARGV[3], ARGV[5])
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
		[]string{futureEventsKey(), futureEventKey(instance, event.ScheduleEventID), payloadKey(instance)},
		strconv.FormatInt(event.VisibleAt.UnixMilli(), 10),
		instanceSegment(instance),
		event.ID,
		string(eventData),
		string(payloadEventData),
	).Err()
}

// Remove a scheduled future event. Not cluster-safe.
// KEYS[1] - future event zset key
// KEYS[2] - future event key
// KEYS[3] - instance payload key
var removeFutureEventCmd = redis.NewScript(`
	redis.call("ZREM", KEYS[1], KEYS[2])
	local eventID = redis.call("HGET", KEYS[2], "id")
	redis.call("HDEL", KEYS[3], eventID)
	return redis.call("DEL", KEYS[2])
`)

// removeFutureEvent removes a scheduled future event for the given event. Events are associated via their ScheduleEventID
func removeFutureEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) {
	key := futureEventKey(instance, event.ScheduleEventID)
	removeFutureEventCmd.Run(ctx, p, []string{futureEventsKey(), key, payloadKey(instance)})
}

// Find all due future events. For each event:
// - Look up event data
// - Add to pending event stream for workflow instance
// - Try to queue workflow task for workflow instance
// - Remove event from future event set and delete event data
//
// KEYS[1] - future event set key
// KEYS[2] - workflow task queue stream
// KEYS[3] - workflow task queue set
// ARGV[1] - current timestamp for zrange
//
// Note: this does not work with Redis Cluster since not all keys are passed into the script.
var futureEventsCmd = redis.NewScript(`
	-- Find events which should become visible now
	local events = redis.call("ZRANGE", KEYS[1], "-inf", ARGV[1], "BYSCORE")
	for i = 1, #events do
		local instanceSegment = redis.call("HGET", events[i], "instance")

		-- Add event to pending event stream
		local eventData = redis.call("HGET", events[i], "event")
		local pending_events_key = "pending-events:" .. instanceSegment
		redis.call("XADD", pending_events_key, "*", "event", eventData)

		-- Try to queue workflow task
		local already_queued = redis.call("SADD", KEYS[3], instanceSegment)
		if already_queued ~= 0 then
			redis.call("XADD", KEYS[2], "*", "id", instanceSegment, "data", "")
		end

		-- Delete event hash data
		redis.call("DEL", events[i])
		redis.call("ZREM", KEYS[1], events[i])
	end

	return #events
`)

func scheduleFutureEvents(ctx context.Context, rb *redisBackend) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	queueKeys := rb.workflowQueue.Keys()

	if _, err := futureEventsCmd.Run(ctx, rb.rdb, []string{
		futureEventsKey(),
		queueKeys.StreamKey,
		queueKeys.SetKey,
	}, nowStr).Result(); err != nil && err != redis.Nil {
		return fmt.Errorf("checking future events: %w", err)
	}

	return nil
}
