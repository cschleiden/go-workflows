-- Complete an activity task, add the result event to the workflow instance, and enqueue the workflow task
-- KEYS[1] = activity set key
-- KEYS[2] = activity stream key
-- KEYS[3] = pending events stream key
-- KEYS[4] = payload hash key
-- KEYS[5] = workflow queues set key
-- KEYS[6] = workflow set key (for specific queue)
-- KEYS[7] = workflow stream key (for specific queue)
-- ARGV[1] = task id (activity)
-- ARGV[2] = group name (activity group)
-- ARGV[3] = event id
-- ARGV[4] = event data (json, without attributes)
-- ARGV[5] = payload data (json, can be empty)
-- ARGV[6] = workflow queue group name
-- ARGV[7] = workflow instance segment id

-- Complete the activity task (from queue/complete.lua)
local task = redis.call("XRANGE", KEYS[2], ARGV[1], ARGV[1])
if #task == 0 then
    return nil
end

local id = task[1][2][2]
redis.call("SREM", KEYS[1], id)
redis.call("XACK", KEYS[2], "NOMKSTREAM", ARGV[2], ARGV[1])
redis.call("XDEL", KEYS[2], ARGV[1])

-- Add event to pending events stream for workflow instance
redis.call("XADD", KEYS[3], "*", "event", ARGV[4])

-- Store payload if provided (only if not empty)
if ARGV[5] ~= "" then
    redis.pcall("HSETNX", KEYS[4], ARGV[3], ARGV[5])
end

-- Enqueue workflow task (from queue/enqueue.lua)
redis.call("SADD", KEYS[5], KEYS[6])
local added = redis.call("SADD", KEYS[6], ARGV[7])
if added == 1 then
    redis.call("XADD", KEYS[7], "*", "id", ARGV[7], "data", "")
end

return true
