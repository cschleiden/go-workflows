-- Return a table with the queue name as key and the number of tasks in the queue as value
-- KEYS[1] = stream set key
local res = {}
local r = server.call("SMEMBERS", KEYS[1])
local idx = 1
for i = 1, #r, 1 do
  local queue = r[i]
  local length = server.call("SCARD", queue)
  table.insert(res, queue)
  table.insert(res, length)
end

return res
