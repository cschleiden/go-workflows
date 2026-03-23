-- Find all due future events. For each event:
-- - Look up event data
-- - Add to pending event stream for workflow instance
-- - Try to queue workflow task for workflow instance
-- - Remove event from future event set and delete event data
--
-- KEYS[1] - future event set key
-- ARGV[1] - current timestamp for zrange
-- ARGV[2] - redis key prefix
--
-- Note: this does not work with Redis Cluster since not all keys are passed into the script.
-- Find events which should become visible now
local now = ARGV[1]
local events = server.call("ZRANGE", KEYS[1], "-inf", now, "BYSCORE")
local prefix = ARGV[2]
for i = 1, #events do
  local instanceSegment = server.call("HGET", events[i], "instance")
  local queue = server.call("HGET", events[i], "queue")

  local setKey = prefix .. "task-set:" .. queue .. ":workflows"
  local streamKey = prefix .. "task-stream:" .. queue .. ":workflows"

  -- Try to queue workflow task. If a workflow task is already queued, ignore this event for now.
  local added = server.call("SADD", setKey, instanceSegment)
  if added == 1 then
    server.call("XADD", streamKey, "*", "id", instanceSegment, "data", "")

    -- Add event to pending event stream
    local eventData = server.call("HGET", events[i], "event")
    local pending_events_key = prefix .. "pending-events:" .. instanceSegment
    server.call("XADD", pending_events_key, "*", "event", eventData)

    -- Delete event hash data
    server.call("DEL", events[i])
    server.call("ZREM", KEYS[1], events[i])
  end
end

return #events
