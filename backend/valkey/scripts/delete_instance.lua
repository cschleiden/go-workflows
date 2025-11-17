local instanceKey = KEYS[1]
local pendingEventsKey = KEYS[2]
local historyKey = KEYS[3]
local payloadKey = KEYS[4]
local activeInstanceExecutionKey = KEYS[5]
local instancesByCreationKey = KEYS[6]

local instanceSegment = ARGV[1]

-- Delete all instance-related keys
server.call("DEL", instanceKey, pendingEventsKey, historyKey, payloadKey, activeInstanceExecutionKey)

-- Remove instance from sorted set
return server.call("ZREM", instancesByCreationKey, instanceSegment)
