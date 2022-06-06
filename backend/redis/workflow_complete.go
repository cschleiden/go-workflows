package redis

import "github.com/go-redis/redis/v8"

// KEYS[1] - Instance key
// KEYS[2] - History stream key
// KEYS[3] - Pending events stream key
// KEYS[4] - Workflow task queue SET
// KEYS[5] - Workflow task queue STREAM
// KEYS[6] - Future events key

// ARGV[1] - Workflow instance InstanceID
// ARGV[2] - Executed event data as serialized strings
// ARGV[3] - Last executed message id
// ARGV[4] - Future event data
var completeWorkflowCmd = redis.NewScript(`
	-- Add new events to stream
	local lastMsgID = ""
	for i = 1, #ARGV[2] do
		lastMsgID = redis.call("XADD", KEYS[2], "*", "event", ARGV[2][i])
	end

	-- Remove executed pending events
	redis.call("XTRIM", KEYS[3], "MINID", ARGV[3])
	redis.call("XDEL", KEYS[3], ARGV[3])

  -- Add future events
	for i = 1, #ARGV[4] do
		redis.call("ZADD", KEYS[6], ARGV[1], KEYS[2])
		redis.call("SET", KEYS[2], ARGV[2])
	end

	-- Iterate over workflow events
		-- Add events to workflow streams
		-- Try to queue workflow tasks
	-- TODO!

	-- Update instance
	-- TODO!, switch instance to HSET? then just update here

	-- Queue activity tasks
	-- TODO!
	-- This means queuing tasks in the activity queue.

	-- Unlock workflow task in queue
	-- If there are pending events, queue another workflow task
	local added = redis.call("SADD", KEYS[4], ARGV[1])
	if added == 1 then
		redis.call("XADD", KEYS[5], "*", "id", ARGV[1], "data", ARGV[2])
	end
`)
