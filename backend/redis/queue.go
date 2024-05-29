package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type taskQueue[T any] struct {
	keyPrefix   string
	tasktype    string
	groupName   string
	workerName  string
	queueSetKey string
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

func newTaskQueue[T any](rdb redis.UniversalClient, keyPrefix string, tasktype string) (*taskQueue[T], error) {
	// Ensure the key prefix ends with a colon
	if keyPrefix != "" && keyPrefix[len(keyPrefix)-1] != ':' {
		keyPrefix += ":"
	}

	tq := &taskQueue[T]{
		keyPrefix:   keyPrefix,
		tasktype:    tasktype,
		groupName:   "task-workers",
		workerName:  uuid.NewString(),
		queueSetKey: fmt.Sprintf("%s%s:queues", keyPrefix, tasktype),
	}

	// Pre-load scripts
	cmds := map[string]*redis.StringCmd{
		"recoverCmd":  recoverCmd.Load(context.Background(), rdb),
		"enqueueCmd":  enqueueCmd.Load(context.Background(), rdb),
		"completeCmd": completeCmd.Load(context.Background(), rdb),
		"sizeCmd":     sizeCmd.Load(context.Background(), rdb),
	}

	for name, cmd := range cmds {
		// if testing.Testing() {
		// 	fmt.Println(name, cmd.Val())
		// }

		if cmd.Err() != nil {
			return nil, fmt.Errorf("loading redis script: %v %w", name, cmd.Err())
		}
	}

	return tq, nil
}

func (q *taskQueue[T]) Keys(queue workflow.Queue) KeyInfo {
	return KeyInfo{
		StreamKey: fmt.Sprintf("%stask-stream:%s:%s", q.keyPrefix, queue, q.tasktype),
		SetKey:    fmt.Sprintf("%stask-set:%s:%s", q.keyPrefix, queue, q.tasktype),
	}
}

// Return a table with the queue name as key and the number of tasks in the queue as value
// KEYS[1] = stream set key
var sizeCmd = redis.NewScript(`
	local res = {}
	local r = redis.call("SMEMBERS", KEYS[1])
	local idx = 1
	for i = 1, #r, 1 do
		local queue = r[i]
		local length = redis.call("SCARD", queue)
		table.insert(res, queue)
		table.insert(res, length)
	end

	return res
`)

func (q *taskQueue[T]) Size(ctx context.Context, rdb redis.UniversalClient) (map[workflow.Queue]int64, error) {
	sizeData, err := sizeCmd.Run(ctx, rdb, []string{q.queueSetKey}).Slice()
	if err != nil {
		return nil, fmt.Errorf("getting queue size: %w", err)
	}

	res := map[workflow.Queue]int64{}

	for i := 0; i < len(sizeData); i += 2 {
		queueName := sizeData[i].(string)

		// Parse queue name from key
		queueName = strings.Split(queueName, ":")[2] // queue name is the third part of the key (0-indexed)

		queue := workflow.Queue(queueName)
		size := sizeData[i+1].(int64)
		res[queue] = size
	}

	return res, nil
}

// KEYS[1] = queues set
// KEYS[2] = set
// KEYS[3] = stream
// ARGV[1] = consumer group
// ARGV[2] = caller provided id of the task
// ARGV[3] = additional data to store with the task
var enqueueCmd = redis.NewScript(
	// Prevent duplicates by checking a set first
	`local added = redis.call("SADD", KEYS[2], ARGV[2])
	if added == 1 then
		local streamExists = redis.call("SISMEMBER", KEYS[1], KEYS[2])
		if streamExists == 0 then
			redis.call("XGROUP", "CREATE", KEYS[3], ARGV[1], "0", "MKSTREAM")
			redis.call("SADD", KEYS[1], KEYS[2])
		end

		redis.call("XADD", KEYS[3], "*", "id", ARGV[2], "data", ARGV[3])
	end

	return true
`)

func (q *taskQueue[T]) Enqueue(ctx context.Context, p redis.Pipeliner, queue workflow.Queue, id string, data *T) error {
	ds, err := json.Marshal(data)
	if err != nil {
		return err
	}

	keys := q.Keys(queue)

	enqueueCmd.Run(ctx, p, []string{q.queueSetKey, keys.SetKey, keys.StreamKey}, q.groupName, id, string(ds))

	return nil
}

func (q *taskQueue[T]) Dequeue(ctx context.Context, rdb redis.UniversalClient, queues []workflow.Queue, lockTimeout, timeout time.Duration) (*TaskItem[T], error) {
	// Try to recover abandoned tasks
	task, err := q.recover(ctx, rdb, queues, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("checking for abandoned tasks: %w", err)
	}

	if task != nil {
		return task, nil
	}

	// Check for new tasks
	streamKeys := []string{}
	streamIds := []string{}
	for _, queue := range queues {
		keys := q.Keys(queue)
		streamKeys = append(streamKeys, keys.StreamKey)
		streamIds = append(streamIds, ">")
	}

	// Try to dequeue from all given queues
	ids, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Streams:  append(streamKeys, streamIds...),
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

func (q *taskQueue[T]) Extend(ctx context.Context, p redis.Pipeliner, queue workflow.Queue, taskID string) error {
	// Claiming a message resets the idle timer. Don't use the `JUSTID` variant, we
	// want to increase the retry counter.
	_, err := p.XClaim(ctx, &redis.XClaimArgs{
		Stream:   q.Keys(queue).StreamKey,
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

func (q *taskQueue[T]) Complete(ctx context.Context, p redis.Pipeliner, queue workflow.Queue, taskID string) (*redis.Cmd, error) {
	cmd := completeCmd.Run(ctx, p, []string{
		q.Keys(queue).SetKey,
		q.Keys(queue).StreamKey,
	}, taskID, q.groupName)
	if err := cmd.Err(); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("completing task: %w", err)
	}

	return cmd, nil
}

func (q *taskQueue[T]) Data(ctx context.Context, p redis.Pipeliner, queue workflow.Queue, taskID string) (*TaskItem[T], error) {
	msg, err := p.XRange(ctx, q.Keys(queue).StreamKey, taskID, taskID).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("finding task: %w", err)
	}

	return msgToTaskItem[T](&msg[0])
}

// KEYS[1] = queues set key
// KEYS[2] = stream key
// KEYS[3] = set key
// ARGV[1] = group name
// ARGV[2] = consumer/worker name
// ARGV[3] = min-idle
// ARGV[4] = start
// ARGV[5] = queue name
var recoverCmd = redis.NewScript(`
	local streamExists = redis.call("SISMEMBER", KEYS[1], KEYS[3])
	if streamExists == 0 then
		redis.call("XGROUP", "CREATE", KEYS[2], ARGV[1], "0", 'MKSTREAM')
		redis.call("SADD", KEYS[1], KEYS[3])
	end

	return redis.call("XAUTOCLAIM", KEYS[2], ARGV[1], ARGV[2], ARGV[3], ARGV[4], "COUNT", 1)
`)

func (q *taskQueue[T]) recover(ctx context.Context, rdb redis.UniversalClient, queues []workflow.Queue, idleTimeout time.Duration) (*TaskItem[T], error) {
	for _, queue := range queues {
		keys := q.Keys(queue)

		r, err := recoverCmd.Run(
			ctx, rdb,
			[]string{q.queueSetKey, keys.StreamKey, keys.SetKey},
			q.groupName,
			q.workerName,
			idleTimeout.Milliseconds(),
			"0",
			string(queue),
		).Slice()
		if err != nil {
			if err == redis.Nil {
				continue
			}

			return nil, fmt.Errorf("recovering abandoned task: %w", err)
		}

		if len(r) > 1 {
			msgs := r[1].([]interface{})
			if len(msgs) > 0 {
				msgData := msgs[0].([]interface{})

				id := msgData[0].(string)
				rawValues := msgData[1].([]interface{})
				values := make(map[string]interface{})
				for i := 0; i < len(rawValues); i += 2 {
					key := rawValues[i].(string)
					value := rawValues[i+1].(string)
					values[key] = value
				}

				return msgToTaskItem[T](&redis.XMessage{
					ID:     id,
					Values: values,
				})
			}
		}
	}

	return nil, nil
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
