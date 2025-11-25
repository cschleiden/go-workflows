-- We need TaskIDs for the stream and caller provided IDs for the set. So first look up
-- the ID in the stream using the TaskID, then remove from the set and the stream
-- KEYS[1] = set
-- KEYS[2] = stream
-- ARGV[1] = task id
-- ARGV[2] = group
-- We have to XACK _and_ XDEL here. See https://github.com/redis/redis/issues/5754
local task = server.call("XRANGE", KEYS[2], ARGV[1], ARGV[1])
if #task == 0 then
    return nil
end

local id = task[1][2][2]
server.call("SREM", KEYS[1], id)
server.call("XACK", KEYS[2], "NOMKSTREAM", ARGV[2], ARGV[1])

-- Delete the task here. Overall we'll keep the stream at a small size, so fragmentation
-- is not an issue for us.
server.call("XDEL", KEYS[2], ARGV[1])

return true