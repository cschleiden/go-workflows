package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"
)

var (
	prepareCmd  *valkey.Lua
	enqueueCmd  *valkey.Lua
	completeCmd *valkey.Lua
	recoverCmd  *valkey.Lua
	sizeCmd     *valkey.Lua
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

func newTaskQueue[T any](keyPrefix, tasktype, workerName string) (*taskQueue[T], error) {
	// Ensure the key prefix ends with a colon
	if keyPrefix != "" && keyPrefix[len(keyPrefix)-1] != ':' {
		keyPrefix += ":"
	}

	if workerName == "" {
		workerName = uuid.NewString()
	}

	tq := &taskQueue[T]{
		keyPrefix:   keyPrefix,
		tasktype:    tasktype,
		groupName:   "task-workers",
		workerName:  workerName,
		queueSetKey: fmt.Sprintf("%s%s:queues", keyPrefix, tasktype),
	}

	// Load all Lua scripts
	cmdMapping := map[string]**valkey.Lua{
		"queue/prepare.lua":  &prepareCmd,
		"queue/size.lua":     &sizeCmd,
		"queue/recover.lua":  &recoverCmd,
		"queue/enqueue.lua":  &enqueueCmd,
		"queue/complete.lua": &completeCmd,
	}

	if err := loadScripts(cmdMapping); err != nil {
		return nil, fmt.Errorf("loading Lua scripts: %w", err)
	}

	return tq, nil
}

func (q *taskQueue[T]) Prepare(ctx context.Context, client valkey.Client, queues []workflow.Queue) error {
	var queueStreamKeys []string
	for _, queue := range queues {
		queueStreamKeys = append(queueStreamKeys, q.Keys(queue).StreamKey)
	}

	err := prepareCmd.Exec(ctx, client, queueStreamKeys, []string{q.groupName}).Error()
	if err != nil && !valkey.IsValkeyNil(err) {
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

func (q *taskQueue[T]) Size(ctx context.Context, client valkey.Client) (map[workflow.Queue]int64, error) {
	sizeData, err := sizeCmd.Exec(ctx, client, []string{q.queueSetKey}, []string{}).ToArray()
	if err != nil {
		return nil, fmt.Errorf("getting queue size: %w", err)
	}

	res := map[workflow.Queue]int64{}
	for i := 0; i < len(sizeData); i += 2 {
		queueName, err := sizeData[i].ToString()
		if err != nil {
			return nil, fmt.Errorf("parsing queue name: %w", err)
		}

		queueName = strings.TrimPrefix(queueName, q.keyPrefix)
		queueName = strings.Split(queueName, ":")[1] // queue name is the third part of the key (0-indexed)

		queue := workflow.Queue(queueName)
		size, err := sizeData[i+1].AsInt64()
		if err != nil {
			return nil, fmt.Errorf("parsing queue size: %w", err)
		}

		res[queue] = size
	}

	return res, nil
}

func (q *taskQueue[T]) Enqueue(ctx context.Context, client valkey.Client, queue workflow.Queue, id string, data *T) error {
	ds, err := json.Marshal(data)
	if err != nil {
		return err
	}

	queueStreamInfo := q.Keys(queue)
	if err := enqueueCmd.Exec(ctx, client, []string{q.queueSetKey, queueStreamInfo.SetKey, queueStreamInfo.StreamKey}, []string{q.groupName, id, string(ds)}).Error(); err != nil {
		return fmt.Errorf("enqueueing task: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) Dequeue(ctx context.Context, client valkey.Client, queues []workflow.Queue, lockTimeout, timeout time.Duration) (*TaskItem[T], error) {
	// Try to recover abandoned tasks
	task, err := q.recover(ctx, client, queues, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("checking for abandoned tasks: %w", err)
	}

	if task != nil {
		return task, nil
	}

	// Check for new tasks
	streamKeys := make([]string, 0, len(queues))
	for _, queue := range queues {
		keyInfo := q.Keys(queue)
		streamKeys = append(streamKeys, keyInfo.StreamKey)
	}

	ids := make([]string, len(streamKeys))
	for i := range ids {
		ids[i] = ">"
	}

	// Try to dequeue from all given queues
	cmd := client.B().Xreadgroup().Group(q.groupName, q.workerName).Block(timeout.Milliseconds()).Streams().Key(streamKeys...).Id(ids...)
	results, err := client.Do(ctx, cmd.Build()).AsXRead()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("dequeueing task: %w", err)
	}

	var msgs []valkey.XRangeEntry
	for _, streamResult := range results {
		msgs = append(msgs, streamResult...)
	}

	if len(results) == 0 || len(msgs) == 0 || valkey.IsValkeyNil(err) {
		return nil, nil
	}

	return msgToTaskItem[T](msgs[0])
}

func (q *taskQueue[T]) Extend(ctx context.Context, client valkey.Client, queue workflow.Queue, taskID string) error {
	// Claiming a message resets the idle timer
	err := client.Do(ctx, client.B().Xclaim().Key(q.Keys(queue).StreamKey).Group(q.groupName).Consumer(q.workerName).MinIdleTime("0").Id(taskID).Build()).Error()
	if err != nil {
		// Check if error is due to no data available (nil response)
		if valkey.IsValkeyNil(err) {
			return nil
		}
		return fmt.Errorf("extending lease: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) Complete(ctx context.Context, client valkey.Client, queue workflow.Queue, taskID string) error {
	err := completeCmd.Exec(ctx, client, []string{
		q.Keys(queue).SetKey,
		q.Keys(queue).StreamKey,
	}, []string{taskID, q.groupName}).Error()
	if err != nil && !valkey.IsValkeyNil(err) {
		return fmt.Errorf("completing task: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) recover(ctx context.Context, client valkey.Client, queues []workflow.Queue, idleTimeout time.Duration) (*TaskItem[T], error) {
	var keys []string
	for _, queue := range queues {
		keys = append(keys, q.Keys(queue).StreamKey)
	}

	r, err := recoverCmd.Exec(ctx, client, keys, []string{q.groupName, q.workerName, strconv.FormatInt(idleTimeout.Milliseconds(), 10), "0"}).ToArray()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("recovering abandoned task: %w", err)
	}

	if len(r) > 1 {
		msgs, err := r[1].ToArray()
		if err != nil {
			return nil, fmt.Errorf("recovering abandoned task: %w", err)
		}
		if len(msgs) > 0 && !msgs[0].IsNil() {
			msgData, err := msgs[0].ToArray()
			if err != nil {
				return nil, fmt.Errorf("recovering abandoned task: %w", err)
			}
			id, err := msgData[0].ToString()
			if err != nil {
				return nil, fmt.Errorf("recovering abandoned task: %w", err)
			}
			rawValues, err := msgData[1].ToArray()
			if err != nil {
				return nil, fmt.Errorf("recovering abandoned task: %w", err)
			}
			values := make(map[string]string)
			for i := 0; i < len(rawValues); i += 2 {
				key, err := rawValues[i].ToString()
				if err != nil {
					return nil, fmt.Errorf("recovering abandoned task: %w", err)
				}
				value, err := rawValues[i+1].ToString()
				if err != nil {
					return nil, fmt.Errorf("recovering abandoned task: %w", err)
				}
				values[key] = value
			}

			return msgToTaskItem[T](valkey.XRangeEntry{
				ID:          id,
				FieldValues: values,
			})
		}
	}

	return nil, nil
}

func msgToTaskItem[T any](msg valkey.XRangeEntry) (*TaskItem[T], error) {
	id, idOk := msg.FieldValues["id"]
	data, dataOk := msg.FieldValues["data"]

	var t T
	if dataOk && data != "" {
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			return nil, err
		}
	}

	if !idOk {
		return nil, fmt.Errorf("message missing id field")
	}

	return &TaskItem[T]{
		TaskID: msg.ID,
		ID:     id,
		Data:   t,
	}, nil
}
