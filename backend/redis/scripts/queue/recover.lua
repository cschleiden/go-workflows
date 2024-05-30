-- KEYS[1..n] = queue stream keys
-- ARGV[1] = group name
-- ARGV[2] = consumer/worker name
-- ARGV[3] = min-idle time in ms
-- ARGV[4] = start

-- Try to recover abandoned tasks
for i = 1, #KEYS do
    local stream = KEYS[i]

    -- If stream doesn't exist, create it
    if 0 == redis.call("EXISTS", stream) then
        redis.pcall("XGROUP", "CREATE", stream, ARGV[1], "0", "MKSTREAM")
    else
      local recovered = redis.pcall("XAUTOCLAIM", stream, ARGV[1], ARGV[2], ARGV[3], ARGV[4], "COUNT", 1)

      -- Check for NOGROUP error
      if recovered.err and recovered.err:find("NOGROUP") then
          -- Create consumer group
          redis.pcall("XGROUP", "CREATE", stream, ARGV[1], "0", "MKSTREAM")
      else
        if #recovered > 0 then
            if #recovered[1] > 0 then
              return recovered
            end
        end
      end
    end
end