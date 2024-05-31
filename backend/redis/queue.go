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

var (
	prepareCmd  *redis.Script
	enqueueCmd  *redis.Script
	completeCmd *redis.Script
	recoverCmd  *redis.Script
	sizeCmd     *redis.Script
)

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

func newTaskQueue[T any](ctx context.Context, rdb redis.UniversalClient, keyPrefix string, tasktype string) (*taskQueue[T], error) {
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

	// Load all Lua scripts
	cmdMapping := map[string]**redis.Script{
		"queue/prepare.lua":  &prepareCmd,
		"queue/enqueue.lua":  &enqueueCmd,
		"queue/recover.lua":  &recoverCmd,
		"queue/complete.lua": &completeCmd,
		"queue/size.lua":     &sizeCmd,
	}

	if err := loadScripts(ctx, rdb, cmdMapping); err != nil {
		return nil, fmt.Errorf("loading Lua scripts: %w", err)
	}

	return tq, nil
}

func (q *taskQueue[T]) Prepare(ctx context.Context, rdb redis.UniversalClient, queues []workflow.Queue) error {
	keys := []string{}
	for _, queue := range queues {
		keys = append(keys, q.Keys(queue).StreamKey)
	}

	_, err := prepareCmd.Run(ctx, rdb, keys, q.groupName).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("preparing queues: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) Keys(queue workflow.Queue) KeyInfo {
	return KeyInfo{
		StreamKey: fmt.Sprintf("%stask-stream:%s:%s", q.keyPrefix, queue, q.tasktype),
		SetKey:    fmt.Sprintf("%stask-set:%s:%s", q.keyPrefix, queue, q.tasktype),
	}
}

func (q *taskQueue[T]) Size(ctx context.Context, rdb redis.UniversalClient) (map[workflow.Queue]int64, error) {
	sizeData, err := sizeCmd.Run(ctx, rdb, []string{q.queueSetKey}).Slice()
	if err != nil {
		return nil, fmt.Errorf("getting queue size: %w", err)
	}

	res := map[workflow.Queue]int64{}

	for i := 0; i < len(sizeData); i += 2 {
		queueName := sizeData[i].(string)

		// Parse queue name from key
		queueName = strings.TrimPrefix(queueName, q.keyPrefix)
		queueName = strings.Split(queueName, ":")[1] // queue name is the third part of the key (0-indexed)

		queue := workflow.Queue(queueName)
		size := sizeData[i+1].(int64)
		res[queue] = size
	}

	return res, nil
}

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

func (q *taskQueue[T]) recover(ctx context.Context, rdb redis.UniversalClient, queues []workflow.Queue, idleTimeout time.Duration) (*TaskItem[T], error) {
	keys := []string{}

	for _, queue := range queues {
		keys = append(keys, q.Keys(queue).StreamKey)
	}

	r, err := recoverCmd.Run(
		ctx, rdb,
		keys,
		q.groupName,
		q.workerName,
		idleTimeout.Milliseconds(),
		"0",
	).Slice()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
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
