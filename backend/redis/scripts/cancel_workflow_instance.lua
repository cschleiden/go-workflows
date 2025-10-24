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
