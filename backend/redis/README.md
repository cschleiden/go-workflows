# Redis backend

Storing:

### Instances and their state

Instances and their state are stored as STRINGS under the `instances-{instanceID}` key.

### Pending events & Locked instance

Pending events are stored in a LIST under the `pending-{instanceID}` key. In addition, whenever an instance receives a new pending event, it's added to a sorted set of instances with pending events.

The `SCORE` for the sorted set is the timestamp when the instance unlocks. For tasks that haven't been picked up, the `SCORE` is 0.

### Timer events

Timer events are stored in a sorted set. Whenever a client checks for a new workflow instance task, the sorted set is checked to see if any of the pending timer events is ready yet. If it is, it's added to the pending events before those are checked for pending workflow tasks.

### History

History is stored in a LIST under the `history-{instanceID}` key.