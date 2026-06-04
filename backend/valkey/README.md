# Valkey backend

The Valkey backend implements the go-workflows `Backend` interface using [Valkey](https://valkey.io/), an open-source Redis-compatible data store. It mirrors the Redis backend in structure and key layout.

## Instances and their state

Instances and their state (started_at, completed_at, etc.) are stored as JSON blobs under the `instances-{instanceID}` keys.

## History and pending events

Events are stored in streams per workflow instance under the `events-{instanceID}` key. A cursor in the instance state tracks the last executed event. Every event after the cursor in the stream is a pending event and will be returned to the worker in the next workflow task.

## Timer events

Timer events are stored in a sorted set (`ZSET`). Whenever a worker checks for a new workflow instance task, the sorted set is checked to see if any pending timer events are ready. Ready events are added to the pending events before being returned for pending workflow tasks.

## Task queues

Queues are needed for activities and workflow instances. In both cases tasks are enqueued, workers poll for work, and every task must eventually be processed — including recovery when a worker crashes after dequeuing but before completing.

Task queues are implemented using Valkey STREAMs. For queues where only a single instance of a task should be in the queue at a time, an additional `SET` is maintained alongside the stream.

<details>
  <summary>Alternatives considered</summary>

  **Option 1 - `ZSET`**:

  Store queue item keys in a `ZSET` scored by the unlock timestamp. Workers query `ZRANGE` for tasks with scores in `-inf, now)` and update the score to `now + lock_timeout` on pickup. No blocking commands available.

  **Option 2 - `LISTS`**:

  Two `LIST`s (pending and processing) with a `ZSET` for lock expiry. Atomic task pickup via `BLMOVE`. Requires periodic scans of the processing list to recover abandoned tasks.

</details>

## Lua scripts

All multi-key atomic operations are implemented as Lua scripts embedded in `scripts/`. The scripts use `server.call` / `server.pcall` (the Valkey-native namespace) rather than the legacy `redis.*` aliases.

**Note**: The Valkey backend does not support Valkey Cluster because the Lua scripts access keys that span multiple hash slots. All keys must reside on a single Valkey node or use a key prefix that maps to the same slot.

## Auto-expiration

Finished workflow instances can be automatically expired after a configurable duration using `WithAutoExpiration`. Continued-as-new instances can use a separate expiry via `WithAutoExpirationContinueAsNew`. Expiration is implemented via a sorted set of expiry timestamps; cleanup runs as a side effect of workflow task completion.
