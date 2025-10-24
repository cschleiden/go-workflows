-- Signal a workflow instance by adding an event to its pending events stream and queuing it
--
-- KEYS[1] - payload hash key
-- KEYS[2] - pending events stream key
-- KEYS[3] - workflow task set key
-- KEYS[4] - workflow task stream key
--
-- ARGV[1] - event id
-- ARGV[2] - event data (JSON)
-- ARGV[3] - event payload (JSON)
-- ARGV[4] - instance segment

local payloadHashKey = KEYS[1]
local pendingEventsKey = KEYS[2]
local workflowSetKey = KEYS[3]
local workflowStreamKey = KEYS[4]

local eventId = ARGV[1]
local eventData = ARGV[2]
local payload = ARGV[3]
local instanceSegment = ARGV[4]

-- Add event payload
redis.pcall("HSETNX", payloadHashKey, eventId, payload)

-- Add event to pending events stream
redis.call("XADD", pendingEventsKey, "*", "event", eventData)

-- Queue workflow task
local added = redis.call("SADD", workflowSetKey, instanceSegment)
if added == 1 then
    redis.call("XADD", workflowStreamKey, "*", "id", instanceSegment, "data", "")
end

return true
