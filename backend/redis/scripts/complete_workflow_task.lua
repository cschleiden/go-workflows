local keyIdx = 1
local argvIdx = 1

local getKey = function()
    local key = KEYS[keyIdx]
    keyIdx = keyIdx + 1
    return key
end

local getArgv = function()
    local argv = ARGV[argvIdx]
    argvIdx = argvIdx + 1
    -- redis.call("ECHO", argv)
    return argv
end

-- Shared keys
local instanceKey = getKey()
local historyStreamKey = getKey()
local pendingEventsKey = getKey()
local payloadHashKey = getKey()
local futureEventZSetKey = getKey()
local activeInstancesKey = getKey()

local workflowSetKey = getKey()
local workflowStreamKey = getKey()

local instanceSegment = getArgv()

local storePayload = function(eventId, payload)
    redis.pcall("HSETNX", payloadHashKey, eventId, payload)
end

-- Read instance
local instance = cjson.decode(redis.call("GET", instanceKey))

-- Add executed events to history
local executedEvents = tonumber(getArgv())
local lastSequenceId = 0
for i = 1, executedEvents do
    local eventId = getArgv()
    local historyId = getArgv()
    local eventData = getArgv()
    local payloadData = getArgv()
    local sequenceId = getArgv()

    -- Add event to history
    redis.call("XADD", historyStreamKey, historyId, "event", eventData)

    storePayload(eventId, payloadData)

    lastSequenceId = tonumber(sequenceId)
end

-- Remove canceled timers
local timersToCancel = tonumber(getArgv())
for i = 1, timersToCancel do
    local futureEventKey = getKey()

    redis.call("ZREM", futureEventZSetKey, futureEventKey)
    -- remove payload
    local eventId = redis.call("HGET", futureEventKey, "id")
    redis.call("HDEL", payloadHashKey, eventId)
    -- remove event hash
    redis.call("DEL", futureEventKey)
end

-- Schedule timers
local timersToSchedule = tonumber(getArgv())
for i = 1, timersToSchedule do
    local eventId = getArgv()
    local timestamp = getArgv()
    local eventData = getArgv()
    local payloadData = getArgv()

    local futureEventKey = getKey()

    redis.call("ZADD", futureEventZSetKey, timestamp, futureEventKey)
	redis.call("HSET", futureEventKey, "instance", instanceSegment, "id", eventId, "event", eventData)
	storePayload(eventId, payloadData)
end

-- Send events to other workflow instances
local otherWorkflowInstances = tonumber(getArgv())
for i = 1, otherWorkflowInstances do
    local targetInstanceKey = getKey()
    local targetActiveInstanceExecutionKey = getKey()

    local targetInstanceSegment = getArgv()
    local targetInstanceId = getArgv()
    local createNewInstance = tonumber(getArgv())
    local eventsToDeliver = tonumber(getArgv())
    local skipEvents = false

    -- Creating a new instance?
    if createNewInstance == 1 then
        local targetInstanceState = getArgv()
        local targetActiveInstanceExecutionState = getArgv()

        local conflictEventId = getArgv()
        local conflictEventData = getArgv()
        local conflictEventPayloadData = getArgv()

        -- Does the instance exist already?
        local instanceExists = redis.call("EXISTS", targetActiveInstanceExecutionState)
        if instanceExists == 1 then
            redis.call("XADD", pendingEventsKey, "*", "event", conflictEventData)
            storePayload(conflictEventId, conflictEventPayloadData)
            redis.call("ECHO", "Conflict detected, event " .. conflictEventId .. " was not delivered to instance " .. targetInstanceSegment .. ".")

            skipEvents = true
        else
            -- Create new instance
            redis.call("SETNX", targetInstanceKey, targetInstanceState)

            -- Set active execution
            redis.call("SET", targetActiveInstanceExecutionKey, targetActiveInstanceExecutionState)

            -- Track active instance
            redis.call("SADD", activeInstancesKey, targetInstanceSegment)
        end
    end

    local instancePendingEventsKey = getKey()
    local instancePayloadHashKey = getKey()
    for j = 1, eventsToDeliver do
        local eventId = getArgv()
        local eventData = getArgv()
        local payloadData = getArgv()

        if not skipEvents then
            -- Add event to pending events
            redis.call("XADD", instancePendingEventsKey, "*", "event", eventData)

            -- Store payload
            redis.pcall("HSETNX", instancePayloadHashKey, eventId, payloadData)
        end
    end

    -- If events were delivered, try to queue a workflow task
    if eventsToDeliver > 0 and not skipEvents then
        -- Enqueue workflow task
        local added = redis.call("SADD", workflowSetKey, targetInstanceSegment)
        if added == 1 then
            redis.call("XADD", workflowStreamKey, "*", "id", targetInstanceSegment, "data", "")
        end
    end
end

-- Update instance state
local now = getArgv()
local state = tonumber(getArgv())

-- State constants
local ContinuedAsNew = tonumber(getArgv())
local Finished = tonumber(getArgv())

instance["state"] = state

-- If workflow instance finished, remove active execution
local activeInstanceExecutionKey = getKey()
if state == ContinuedAsNew or state == Finished then
    -- Remove active execution
    redis.call("DEL", activeInstanceExecutionKey)

    instance["completed_at"] = now

    -- TODO: Set auto expiration

    redis.call("SREM", activeInstancesKey, instanceSegment)
end

if lastSequenceId > 0 then
    instance["last_sequence_id"] = lastSequenceId
end

redis.call("SET", instanceKey, cjson.encode(instance))

-- Schedule activities
local activities = tonumber(getArgv())
local activitySetKey = getKey()
local activityStreamKey = getKey()
for i = 1, activities do
    local activityId = getArgv()
    local activityData = getArgv()

    local added = redis.call("SADD", activitySetKey, activityId)
	if added == 1 then
		redis.call("XADD", activityStreamKey, "*", "id", activityId, "data", activityData)
	end
end

-- Remove executed pending events
local lastPendingEventMessageId = getArgv()
redis.call("XTRIM", pendingEventsKey, "MINID", lastPendingEventMessageId)
redis.call("XDEL", pendingEventsKey, lastPendingEventMessageId)

-- Complete workflow task and unlock instance
local taskId = getArgv()
local groupName = getArgv()
local task = redis.call("XRANGE", workflowStreamKey, taskId, taskId)
if #task ~= 0 then
    local id = task[1][2][2]
    redis.call("SREM", workflowSetKey, id)
    redis.call("XACK", workflowStreamKey, groupName, taskId)

    redis.call("XDEL", workflowStreamKey, taskId)
end

-- If there are pending events, queue the instance again
local pending_events = redis.call("XLEN", pendingEventsKey)
if pending_events > 0 then
    local added = redis.call("SADD", workflowSetKey, instanceSegment)
    if added == 1 then
        redis.call("XADD", workflowStreamKey, "*", "id", instanceSegment, "data", "")
    end
end

return true