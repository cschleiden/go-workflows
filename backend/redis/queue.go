package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type taskQueue[T any] struct {
	tasktype   string
	setKey     string
	streamKey  string
	groupName  string
	workerName string
}

type TaskItem[T any] struct {
	// TaskID is the generated ID of the task item
	TaskID string

	// ID is the provided id
	ID string

	// Optional data stored with a task, needs to be serializable
	Data T
}

type KeyInfo struct {
	StreamKey string
	SetKey    string
}

func newTaskQueue[T any](rdb redis.UniversalClient, tasktype string) (*taskQueue[T], error) {
	tq := &taskQueue[T]{
		tasktype:   tasktype,
		setKey:     "task-set:" + tasktype,
		streamKey:  "task-stream:" + tasktype,
		groupName:  "task-workers",
		workerName: uuid.NewString(),
	}

	// Pre-load script
	cmds := map[string]*redis.StringCmd{
		"createGroupCmd": createGroupCmd.Load(context.Background(), rdb),
		"enqueueCmd":     enqueueCmd.Load(context.Background(), rdb),
		"completeCmd":    completeCmd.Load(context.Background(), rdb),
	}

	for name, cmd := range cmds {
		// fmt.Println(name, cmd.Val())

		if cmd.Err() != nil {
			return nil, fmt.Errorf("loading redis script: %v %w", name, cmd.Err())
		}
	}

	// Create the consumer group
	err := createGroupCmd.Run(context.Background(), rdb, []string{tq.streamKey, tq.groupName}).Err()
	if err != nil {
		return nil, fmt.Errorf("creating task queue: %w", err)
	}

	return tq, nil
}

func (q *taskQueue[T]) Keys() KeyInfo {
	return KeyInfo{
		StreamKey: q.streamKey,
		SetKey:    q.setKey,
	}
}

func (q *taskQueue[T]) Size(ctx context.Context, rdb redis.UniversalClient) (int64, error) {
	return rdb.XLen(ctx, q.streamKey).Result()
}

// KEYS[1] = set
// KEYS[2] = stream
// ARGV[1] = caller provided id of the task
// ARGV[2] = additional data to store with the task
var enqueueCmd = redis.NewScript(
	// Prevent duplicates by checking a set first
	`local added = redis.call("SADD", KEYS[1], ARGV[1])
	if added == 1 then
		redis.call("XADD", KEYS[2], "*", "id", ARGV[1], "data", ARGV[2])
	end

	return true
`)

var createGroupCmd = redis.NewScript(`
    local streamKey = KEYS[1]
    local groupName = KEYS[2]
    local exists = false
    local res = redis.pcall('XINFO', 'GROUPS', streamKey)

    if res and type(res) == 'table' then
        for _, groupInfo in ipairs(res) do
            if type(groupInfo) == 'table' then
                for i = 1, #groupInfo, 2 do
                    if groupInfo[i] == 'name' and groupInfo[i+1] == groupName then
                        exists = true
                        break
                    end
                end
            end
            if exists then break end
        end
    end

    if not exists then
        redis.call('XGROUP', 'CREATE', streamKey, groupName, '$', 'MKSTREAM')
    end

    return true
`)


func (q *taskQueue[T]) Enqueue(ctx context.Context, p redis.Pipeliner, id string, data *T) error {
	ds, err := json.Marshal(data)
	if err != nil {
		return err
	}

	enqueueCmd.Run(ctx, p, []string{q.setKey, q.streamKey}, id, string(ds))

	return nil
}

func (q *taskQueue[T]) Dequeue(ctx context.Context, rdb redis.UniversalClient, lockTimeout, timeout time.Duration) (*TaskItem[T], error) {
	// Try to recover abandoned messages
	task, err := q.recover(ctx, rdb, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("checking for abandoned tasks: %w", err)
	}

	if task != nil {
		return task, nil
	}

	// Check for new tasks
	ids, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Streams:  []string{q.streamKey, ">"},
		Group:    q.groupName,
		Consumer: q.workerName,
		Count:    1,
		Block:    timeout,
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("dequeueing task: %w", err)
	}

	if len(ids) == 0 || len(ids[0].Messages) == 0 || err == redis.Nil {
		return nil, nil
	}

	msg := ids[0].Messages[0]
	return msgToTaskItem[T](&msg)
}

func (q *taskQueue[T]) Extend(ctx context.Context, p redis.Pipeliner, taskID string) error {
	// Claiming a message resets the idle timer. Don't use the `JUSTID` variant, we
	// want to increase the retry counter.
	_, err := p.XClaim(ctx, &redis.XClaimArgs{
		Stream:   q.streamKey,
		Group:    q.groupName,
		Consumer: q.workerName,
		Messages: []string{taskID},
		MinIdle:  0, // Always claim this message
	}).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("extending lease: %w", err)
	}

	return nil
}

// We need TaskIDs for the stream and caller provided IDs for the set. So first look up
// the ID in the stream using the TaskID, then remove from the set and the stream
// KEYS[1] = set
// KEYS[2] = stream
// ARGV[1] = task id
// ARGV[2] = group
// We have to XACK _and_ XDEL here. See https://github.com/redis/redis/issues/5754
var completeCmd = redis.NewScript(
	`local task = redis.call("XRANGE", KEYS[2], ARGV[1], ARGV[1])
	if #task == 0 then
		return nil
	end
	local id = task[1][2][2]
	redis.call("SREM", KEYS[1], id)
	redis.call("XACK", KEYS[2], ARGV[2], ARGV[1])

	-- Delete the task here. Overall we'll keep the stream at a small size, so fragmentation
	-- is not an issue for us.
	return redis.call("XDEL", KEYS[2], ARGV[1])
`)

func (q *taskQueue[T]) Complete(ctx context.Context, p redis.Pipeliner, taskID string) (*redis.Cmd, error) {
	cmd := completeCmd.Run(ctx, p, []string{q.setKey, q.streamKey}, taskID, q.groupName)
	if err := cmd.Err(); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("completing task: %w", err)
	}

	return cmd, nil
}

func (q *taskQueue[T]) Data(ctx context.Context, p redis.Pipeliner, taskID string) (*TaskItem[T], error) {
	msg, err := p.XRange(ctx, q.streamKey, taskID, taskID).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("finding task: %w", err)
	}

	return msgToTaskItem[T](&msg[0])
}

func (q *taskQueue[T]) recover(ctx context.Context, rdb redis.UniversalClient, idleTimeout time.Duration) (*TaskItem[T], error) {
	// Ignore the start argument, we are deleting tasks as they are completed, so we'll always
	// start this scan from the beginning.
	msgs, _, err := rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.streamKey,
		Group:    q.groupName,
		Consumer: q.workerName,
		MinIdle:  idleTimeout,
		Count:    1,   // Get at most one abandoned task
		Start:    "0", // Start at the beginning of the pending items
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("recovering tasks: %w", err)
	}

	if len(msgs) == 0 {
		return nil, nil
	}

	return msgToTaskItem[T](&msgs[0])
}

func msgToTaskItem[T any](msg *redis.XMessage) (*TaskItem[T], error) {
	id := msg.Values["id"].(string)
	data := msg.Values["data"].(string)

	var t T
	if data != "" {
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			return nil, err
		}
	}

	return &TaskItem[T]{
		TaskID: msg.ID,
		ID:     id,
		Data:   t,
	}, nil
}
