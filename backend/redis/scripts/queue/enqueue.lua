-- KEYS[1] = queues set
-- KEYS[2] = set
-- KEYS[3] = stream
-- ARGV[1] = consumer group
-- ARGV[2] = caller provided id of the task
-- ARGV[3] = additional data to store with the task
redis.call("SADD", KEYS[1], KEYS[2])
local added = redis.call("SADD", KEYS[2], ARGV[2])
if added == 1 then
  redis.call("XADD", KEYS[3], "*", "id", ARGV[2], "data", ARGV[3])
end

return true