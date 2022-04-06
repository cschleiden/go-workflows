# Redis backend

## Data

### Instances and their state

Instances and their state (started_at, completed_at etc.) are stored as JSON blobs under the `instances-{instanceID}` keys.

### History Events

Events are stored in `STREAM`s per workflow instance under `events-{instanceID}`.

### Pending events & pending instances

Pending events are stored in `STREAM`s under the `pending-{instanceID}` key.

Instances ready to be picked up are stored in a `ZSET`. The `SCORE` for the sorted set is the timestamp when the instance unlocks. For newly added tasks, that haven't been picked up, the `SCORE` is the current timestamp.

Workers make a `ZRANGE` query to that sorted set, looking for instances where the score is in `-inf, now)`. That will get instances where the lock timestamp is in the past. Either because they have just been queued, or the lock has expired.

Once a worker picks up an instance, the score is updated to `now + lock_timeout`. Query and update are done in a transaction, although that could be done as a lua script.

When a worker is done with a task, it removes it from the `ZSET` (`ZREM`).

_Note:_ the queue of pending items could also be another `STREAM`?

### Timer events

Timer events are stored in a sorted set. Whenever a client checks for a new workflow instance task, the sorted set is checked to see if any of the pending timer events is ready yet. If it is, it's added to the pending events before those are checked for pending workflow tasks.

### Activities

#### Option 1: `ZSET`

We need a queue of activities for workers to pick up. We also need to make sure that every activity is eventually processed. So if a worker crashes while processing an activity, we eventually want another worker to pick up the activity.

Activity data is stored JSON serialized under `activity-{activityID}` keys. `activityID`s are added to a ZSET `activities`. The score for the sorted set is the timestamp when the activity task is unlocked. For new activities the `SCORE` is the current timestamp.

Workers make a `ZRANGE` query to that sorted set, looking for activities where the `SCORE` is in `-inf, now)`. That will get activites where the lock timestamp is in the past. Either because they have just been queued, or the lock has expired.

Once a worker picks up an activity, the score is updated to `now + lock_timeout`. Query and update are done in a transaction, although that could be done as a lua script.

When a worker is done with an activity, it removes it from the `ZSET` (`ZREM`), the `actitity-{activityID}` key, adds the resulting event to the pending events `STREAM`, and marks the instance as pending - if not already pending or being processed.

Pro:
- No special handling for recovering crashed activities

Con:
- Need for polling and cannot use any of the blocking redis commands
- WAIT with transaction required

####

Alternatively, we could use two `LIST`s, one for the _pending_ queue, one for the _processing_ queue. With a single command `BLMOVE .. RIGHT  LEFT` we could atomically move and return elements. We'd still

So get task:
1. `BLMOVE ...` from _pending_ to _processing_ queue

TODO: How to track the time when an item was moved between the two? Instead of using references in the list, use fully serialized objects - including the timestamp? How can we extend that for long running activities. POP and add BACK?

Keep storing state externally. Use the queue only for references. Always ensure a task is _first_ stored as a KEY and _then_ added to the queue.
