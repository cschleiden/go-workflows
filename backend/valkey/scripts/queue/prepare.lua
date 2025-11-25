-- KEYS[1..n] - queue stream keys
-- ARGV[1] - group name

for i = 1, #KEYS do
  local streamKey = KEYS[i]
  local groupName = ARGV[1]
  local exists = false
  local res = redis.pcall('XINFO', 'GROUPS', streamKey)

  if res and type(res) == 'table' then
    for _, groupInfo in ipairs(res) do
      if type(groupInfo) == 'table' then
        for i = 1, #groupInfo, 2 do
          if groupInfo[i] == 'name' and groupInfo[i + 1] == groupName then
            exists = true
            break
          end
        end
      end

      if exists then
          break
      end
    end
  end

  if not exists then
    server.call('XGROUP', 'CREATE', streamKey, groupName, '0', 'MKSTREAM')
  end
end