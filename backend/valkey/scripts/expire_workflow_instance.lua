-- Set the given expiration time on all keys passed in
-- KEYS[1] - instances-by-creation key
-- KEYS[2] - instances-expiring key
-- KEYS[3] - instance key
-- KEYS[4] - pending events key
-- KEYS[5] - history key
-- KEYS[6] - payload key
-- ARGV[1] - current timestamp
-- ARGV[2] - expiration time in seconds
-- ARGV[3] - expiration timestamp in unix milliseconds
-- ARGV[4] - instance segment

-- Find instances which have already expired and remove from the index set
local expiredInstances = server.call("ZRANGE", KEYS[2], "-inf", ARGV[1], "BYSCORE")
for i = 1, #expiredInstances do
  local instanceSegment = expiredInstances[i]
  server.call("ZREM", KEYS[1], instanceSegment) -- index set
  server.call("ZREM", KEYS[2], instanceSegment) -- expiration set
end

-- Add expiration time for future cleanup
server.call("ZADD", KEYS[2], ARGV[3], ARGV[4])

-- Set expiration on all keys
for i = 3, #KEYS do
  server.call("EXPIRE", KEYS[i], ARGV[2])
end

return 0
