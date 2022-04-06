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
	groupName  string
	workerName string
	streamName string
}

type TaskItem[T any] struct {
	// TaskID is the generated ID of the task item
	TaskID string

	// ID is the provided id
	ID string

	// Optional data stored with a task, needs to be serializable
	Data T
}

type TaskQueue[T any] interface {
	Enqueue(ctx context.Context, id string, data *T) error
	Dequeue(ctx context.Context, lockTimeout, timeout time.Duration) (*TaskItem[T], error)
	Extend(ctx context.Context, taskID string) error
	Complete(ctx context.Context, id string) error
}

func New[T any](rdb redis.UniversalClient, tasktype string) (TaskQueue[T], error) {
	tq := &taskQueue[T]{
		tasktype:   tasktype,
		rdb:        rdb,
		groupName:  "task-workers",
		workerName: uuid.NewString(),
		streamName: "task-stream:" + tasktype,
	}

	// Create the consumer group
	_, err := tq.rdb.XGroupCreateMkStream(context.Background(), tq.streamName, tq.groupName, "0").Result()
	if err != nil {
		// Ugly, check since there is no UPSERT for consumer groups. Might replace with a script
		// using XINFO & XGROUP CREATE atomically
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return nil, errors.Wrap(err, "could not create task queue")
		}
	}

	return tq, nil
}

func (q *taskQueue[T]) Enqueue(ctx context.Context, id string, data *T) error {
	ds, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamName,
		Values: map[string]interface{}{
			"id":   id,
			"data": string(ds),
		},
	}).Result()
	if err != nil {
		return errors.Wrap(err, "could not enqueue task")
	}

	return nil
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
		Streams:  []string{q.streamName, ">"},
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
		Stream:   q.streamName,
		Group:    q.groupName,
		Consumer: q.workerName,
		Messages: []string{taskID},
		MinIdle:  0, // Always claim this message
	}).Result()
	if err != nil && err != redis.Nil {
		return errors.Wrap(nil, "could not extend lease")
	}

	return nil
}

func (q *taskQueue[T]) Complete(ctx context.Context, id string) error {
	// c, err := q.rdb.XAck(ctx, q.streamName, q.groupName, id).Result()
	// if err != nil {
	// 	return errors.Wrap(nil, "could not complete task")
	// }

	// if c != 1 {
	// 	return errors.New("could find task to complete")
	// }

	// Delete the task here. Overall we'll keep the stream at a small size, so fragmentation
	// is not an issue for us.
	c, err := q.rdb.XDel(ctx, q.streamName, id).Result()
	if err != nil {
		return errors.Wrap(nil, "could not complete task")
	}

	if c != 1 {
		return errors.New("could find task to complete")
	}

	return nil
}

func (q *taskQueue[T]) recover(ctx context.Context, idleTimeout time.Duration) (*TaskItem[T], error) {
	// Ignore the start argument, we are deleting tasks as they are completed, so we'll always
	// start this scan from the beginning.
	msgs, _, err := q.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.streamName,
		Group:    q.groupName,
		Consumer: q.workerName,
		MinIdle:  idleTimeout,
		Count:    1,   // Get at most one abandoned task
		Start:    "0", // Start at the beginning of the pending items
	}).Result()

	if err != nil {
		return nil, errors.Wrap(nil, "could not recover tasks")
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
