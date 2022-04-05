# Redis backend

## Data

### Instances and their state

Instances and their state are stored as HASHES under the `instances-{instanceID}` key.

### Events

Events are stored

### Pending events & Locked instance

Pending events are stored in a LIST under the `pending-{instanceID}` key. In addition, whenever an instance receives a new pending event, it's added to a sorted set of instances with pending events.

The `SCORE` for the sorted set is the timestamp when the instance unlocks. For tasks that haven't been picked up, the `SCORE` is 0.

### Timer events

Timer events are stored in a sorted set. Whenever a client checks for a new workflow instance task, the sorted set is checked to see if any of the pending timer events is ready yet. If it is, it's added to the pending events before those are checked for pending workflow tasks.

### History

History is stored in a LIST under the `history-{instanceID}` key.

## Operations

When a new instance is created, its state is stored as a `HASH` under `instances-{instanceID}`. Its event is added to a new `STREAM`