package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

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
	local now = ARGV[1]
	local events = redis.call("ZRANGE", KEYS[1], "-inf", now, "BYSCORE")
	for i = 1, #events do
		local instanceSegment = redis.call("HGET", events[i], "instance")

		-- Try to queue workflow task. If a workflow task is already queued, ignore this event for now.
		local added = redis.call("SADD", KEYS[3], instanceSegment)
		if added == 1 then
			redis.call("XADD", KEYS[2], "*", "id", instanceSegment, "data", "")

			-- Add event to pending event stream
			local eventData = redis.call("HGET", events[i], "event")
			local pending_events_key = "pending-events:" .. instanceSegment
			redis.call("XADD", pending_events_key, "*", "event", eventData)

			-- Delete event hash data
			redis.call("DEL", events[i])
			redis.call("ZREM", KEYS[1], events[i])
		end
	end

	return #events
`)

func scheduleFutureEvents(ctx context.Context, rb *redisBackend) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	queueKeys := rb.workflowQueue.Keys()

	if _, err := futureEventsCmd.Run(ctx, rb.rdb, []string{
		rb.keys.futureEventsKey(),
		queueKeys.StreamKey,
		queueKeys.SetKey,
	}, nowStr).Result(); err != nil && err != redis.Nil {
		return fmt.Errorf("checking future events: %w", err)
	}

	return nil
}
