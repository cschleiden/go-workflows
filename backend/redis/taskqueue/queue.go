package taskqueue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type taskQueue[T any] struct {
	tasktype   string
	rdb        redis.UniversalClient
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

var ErrTaskAlreadyInQueue = errors.New("task already in queue")

type TaskQueue[T any] interface {
	Enqueue(ctx context.Context, id string, data *T) (*string, error)
	Dequeue(ctx context.Context, lockTimeout, timeout time.Duration) (*TaskItem[T], error)
	Extend(ctx context.Context, taskID string) error
	Complete(ctx context.Context, taskID string) error
	Data(ctx context.Context, taskID string) (*TaskItem[T], error)
}

func New[T any](rdb redis.UniversalClient, tasktype string) (TaskQueue[T], error) {
	tq := &taskQueue[T]{
		tasktype:   tasktype,
		rdb:        rdb,
		setKey:     "task-set:" + tasktype,
		streamKey:  "task-stream:" + tasktype,
		groupName:  "task-workers",
		workerName: uuid.NewString(),
	}

	// Create the consumer group
	_, err := tq.rdb.XGroupCreateMkStream(context.Background(), tq.streamKey, tq.groupName, "0").Result()
	if err != nil {
		// Ugly, check since there is no UPSERT for consumer groups. Might replace with a script
		// using XINFO & XGROUP CREATE atomically
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return nil, errors.Wrap(err, "could not create task queue")
		}
	}

	return tq, nil
}

// KEYS[1] = stream
// KEYS[2] = stream
// ARGV[1] = caller provided id of the task
// ARGV[2] = additional data to store with the task
var enqueueCmd = redis.NewScript(`
	local exists = redis.call("sadd", KEYS[1], ARGV[1])
	if exists == 0 then
		return nil
	end

	return redis.call("xadd", KEYS[2], "*", "id", ARGV[1], "data", ARGV[2])
`)

func (q *taskQueue[T]) Enqueue(ctx context.Context, id string, data *T) (*string, error) {
	ds, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	taskID, err := enqueueCmd.Run(ctx, q.rdb, []string{q.setKey, q.streamKey}, id, string(ds)).Result()
	if err != nil && err != redis.Nil {
		return nil, errors.Wrap(err, "could not enqueue task")
	}

	if taskID == nil {
		return nil, ErrTaskAlreadyInQueue
	}

	tidStr := taskID.(string)
	return &tidStr, nil
}

func (q *taskQueue[T]) Dequeue(ctx context.Context, lockTimeout, timeout time.Duration) (*TaskItem[T], error) {
	// Try to recover abandoned messages
	task, err := q.recover(ctx, lockTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "could check for abandoned tasks")
	}

	if task != nil {
		return task, nil
	}

	// Check for new tasks
	ids, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Streams:  []string{q.streamKey, ">"},
		Group:    q.groupName,
		Consumer: q.workerName,
		Count:    1,
		Block:    timeout,
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, errors.Wrap(err, "could not dequeue task")
	}

	if len(ids) == 0 || len(ids[0].Messages) == 0 || err == redis.Nil {
		return nil, nil
	}

	msg := ids[0].Messages[0]
	return msgToTaskItem[T](&msg)
}

func (q *taskQueue[T]) Extend(ctx context.Context, taskID string) error {
	// Claiming a message resets the idle timer. Don't use the `JUSTID` variant, we
	// want to increase the retry counter.
	_, err := q.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   q.streamKey,
		Group:    q.groupName,
		Consumer: q.workerName,
		Messages: []string{taskID},
		MinIdle:  0, // Always claim this message
	}).Result()
	if err != nil && err != redis.Nil {
		return errors.Wrap(err, "could not extend lease")
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
var completeCmd = redis.NewScript(`
	local task = redis.call("XRANGE", KEYS[2], ARGV[1], ARGV[1])
	if task == nil then
		return nil
	end
	local id = task[1][2][2]
	redis.call("SREM", KEYS[1], id)
	redis.call("XACK", KEYS[2], ARGV[2], ARGV[1])
	return redis.call("XDEL", KEYS[2], ARGV[1])
`)

func (q *taskQueue[T]) Complete(ctx context.Context, taskID string) error {
	// Delete the task here. Overall we'll keep the stream at a small size, so fragmentation
	// is not an issue for us.
	c, err := completeCmd.Run(ctx, q.rdb, []string{q.setKey, q.streamKey}, taskID, q.groupName).Result()
	if err != nil && err != redis.Nil {
		return errors.Wrap(err, "could not complete task")
	}

	if c.(int64) == 0 || err == redis.Nil {
		return errors.New("could not find task to complete")
	}

	return nil
}

func (q *taskQueue[T]) Data(ctx context.Context, taskID string) (*TaskItem[T], error) {
	msg, err := q.rdb.XRange(ctx, q.streamKey, taskID, taskID).Result()
	if err != nil && err != redis.Nil {
		return nil, errors.Wrap(err, "could not find task")
	}

	return msgToTaskItem[T](&msg[0])
}

func (q *taskQueue[T]) recover(ctx context.Context, idleTimeout time.Duration) (*TaskItem[T], error) {
	// Ignore the start argument, we are deleting tasks as they are completed, so we'll always
	// start this scan from the beginning.
	msgs, _, err := q.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.streamKey,
		Group:    q.groupName,
		Consumer: q.workerName,
		MinIdle:  idleTimeout,
		Count:    1,   // Get at most one abandoned task
		Start:    "0", // Start at the beginning of the pending items
	}).Result()

	if err != nil {
		return nil, errors.Wrap(err, "could not recover tasks")
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
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, err
	}

	return &TaskItem[T]{
		TaskID: msg.ID,
		ID:     id,
		Data:   t,
	}, nil
}
