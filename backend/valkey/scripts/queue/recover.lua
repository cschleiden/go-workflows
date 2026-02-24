-- KEYS[1..n] = queue stream keys
-- ARGV[1] = group name
-- ARGV[2] = consumer/worker name
-- ARGV[3] = min-idle time in ms
-- ARGV[4] = start

-- Try to recover abandoned tasks
for i = 1, #KEYS do
  local stream = KEYS[i]
  local recovered = server.call("XAUTOCLAIM", stream, ARGV[1], ARGV[2], ARGV[3], ARGV[4], "COUNT", 1)
  if #recovered > 0 then
      if #recovered[1] > 0 then
        return recovered
      end
  end
end