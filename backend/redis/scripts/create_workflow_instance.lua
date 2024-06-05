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
    return argv
end

local instanceKey = getKey()
local activeInstanceExecutionKey = getKey()
local pendingEventsKey = getKey()
local payloadHashKey = getKey()

local instancesActiveKey = getKey()
local instancesByCreation = getKey()

local workflowSetKey = getKey()
local workflowStreamKey = getKey()
local workflowQueuesSet = getKey()

local instanceSegment = getArgv()

-- Is there an existing instance with active execution?
local instanceExists = redis.call("EXISTS", activeInstanceExecutionKey)
if instanceExists == 1 then
  return redis.error_reply("ERR InstanceAlreadyExists")
end

-- Create new instance
local instanceState = getArgv()
redis.call("SETNX", instanceKey, instanceState)

-- Set active execution
local activeInstanceExecutionState = getArgv()
redis.call("SET", activeInstanceExecutionKey, activeInstanceExecutionState)

-- Track active instance
redis.call("SADD", instancesActiveKey, instanceSegment)

-- add initial event & payload
local eventId = getArgv()
local eventData = getArgv()
redis.call("XADD", pendingEventsKey, "*", "event", eventData)

local payload = getArgv()
redis.pcall("HSETNX", payloadHashKey, eventId, payload)

local creationTimestamp = tonumber(getArgv())
redis.call("ZADD", instancesByCreation, creationTimestamp, instanceSegment)

-- queue workflow task
redis.call("SADD", workflowQueuesSet, workflowSetKey) -- track queue
local added = redis.call("SADD", workflowSetKey, instanceSegment)
if added == 1 then
    redis.call("XADD", workflowStreamKey, "*", "id", instanceSegment, "data", "")
end

return true