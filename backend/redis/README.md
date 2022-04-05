# Redis backend

## Data

### Instances and their state

Instances and their state (started_at, completed_at etc.) are stored as JSON blobs under the `instances-{instanceID}` keys.

### History Events

Events are stored in `STREAM`s per workflow instance under `events-{instanceID}`.

### Pending events & Locked instance

Pending events are stored in `STREAM`s under the `pending-{instanceID}` key.

Instances ready to be picked up are stored in a `ZSET`. The `SCORE` for the sorted set is the timestamp when the instance unlocks. For tasks that haven't been picked up, the `SCORE` is 0.

Workers make `ZRANGE` query to that sorted set, looking for instances where the score is in `-inf, now)`. That will get instances which aren't locked (score = 0) or where the lock has expired (score < now).

Once a worker picks up an instance, the score is updated to `now + lock_timeout`. Query and update are done in a transaction, although that could be done as a lua script.

When a worker is done with a task, it removes it from the `ZSET` (`ZREM`).

_Note:_ the queue of pending items could also be another `STREAM`?

### Timer events

Timer events are stored in a sorted set. Whenever a client checks for a new workflow instance task, the sorted set is checked to see if any of the pending timer events is ready yet. If it is, it's added to the pending events before those are checked for pending workflow tasks.
