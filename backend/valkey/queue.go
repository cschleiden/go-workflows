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

type taskQueue[T any] struct {
	keyPrefix   string
	tasktype    string
	groupName   string
	workerName  string
	queueSetKey string
	// hashTag is a Valkey Cluster hash tag ensuring all keys used together
	// (across different queues for the same task type) map to the same slot.
	// This avoids CrossSlot errors when Valkey is running in clustered/serverless modes
	// and XREADGROUP is called on multiple stream keys.
	hashTag string
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

	// Use a stable Valkey Cluster hash tag so that all keys for this task type
	// hash to the same slot regardless of the specific queue name. Only the
	// substring within {...} is used for hashing.
	// Example generated keys:
	//   <prefix>{task:<tasktype>}:task-stream:<queue>
	//   <prefix>{task:<tasktype>}:task-set:<queue>
	//   <prefix>{task:<tasktype>}:<tasktype>:queues
	hashTag := fmt.Sprintf("{task:%s}", tasktype)

	tq := &taskQueue[T]{
		keyPrefix:   keyPrefix,
		tasktype:    tasktype,
		groupName:   "task-workers",
		workerName:  workerName,
		queueSetKey: fmt.Sprintf("%s%s:%s:queues", keyPrefix, hashTag, tasktype),
		hashTag:     hashTag,
	}

	return tq, nil
}

func (q *taskQueue[T]) Prepare(ctx context.Context, client valkey.Client, queues []workflow.Queue) error {
	for _, queue := range queues {
		streamKey := q.Keys(queue).StreamKey
		err := client.Do(ctx, client.B().XgroupCreate().Key(streamKey).Group(q.groupName).Id("0").Mkstream().Build()).Error()
		if err != nil {
			// Group might already exist, which is fine, consider prepare successful
			if !strings.Contains(err.Error(), "BUSYGROUP") {
				return fmt.Errorf("preparing queue %s: %w", queue, err)
			}
		}
	}

	return nil
}

func (q *taskQueue[T]) Keys(queue workflow.Queue) KeyInfo {
	return KeyInfo{
		StreamKey: fmt.Sprintf("%s%s:task-stream:%s", q.keyPrefix, q.hashTag, queue),
		SetKey:    fmt.Sprintf("%s%s:task-set:%s", q.keyPrefix, q.hashTag, queue),
	}
}

func (q *taskQueue[T]) Size(ctx context.Context, client valkey.Client) (map[workflow.Queue]int64, error) {
	members, err := client.Do(ctx, client.B().Smembers().Key(q.queueSetKey).Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("getting queue size: %w", err)
	}

	res := map[workflow.Queue]int64{}
	for _, queueSetKey := range members {
		size, err := client.Do(ctx, client.B().Scard().Key(queueSetKey).Build()).AsInt64()
		if err != nil {
			return nil, fmt.Errorf("getting queue size: %w", err)
		}

		trimmed := strings.TrimPrefix(queueSetKey, q.keyPrefix)
		lastIdx := strings.LastIndex(trimmed, ":")
		if lastIdx == -1 || lastIdx == len(trimmed)-1 {
			return nil, fmt.Errorf("unexpected set key format: %s", queueSetKey)
		}
		queue := workflow.Queue(trimmed[lastIdx+1:])
		res[queue] = size
	}

	return res, nil
}

func (q *taskQueue[T]) Enqueue(ctx context.Context, client valkey.Client, queue workflow.Queue, id string, data *T) error {
	ds, err := json.Marshal(data)
	if err != nil {
		return err
	}

	keys := q.Keys(queue)

	// Add to set to track uniqueness
	err = client.Do(ctx, client.B().Sadd().Key(q.queueSetKey).Member(keys.SetKey).Build()).Error()
	if err != nil {
		return err
	}

	// Add to set for this queue
	added, err := client.Do(ctx, client.B().Sadd().Key(keys.SetKey).Member(id).Build()).AsInt64()
	if err != nil {
		return err
	}

	// Only add to stream if it's a new task
	if added > 0 {
		err = client.Do(ctx, client.B().Xadd().Key(keys.StreamKey).Id("*").FieldValue().FieldValue("id", id).FieldValue("data", string(ds)).Build()).Error()
		if err != nil {
			return err
		}
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

	// Try to dequeue from all given queues
	cmd := client.B().Xreadgroup().Group(q.groupName, q.workerName).Streams().Key(streamKeys...).Id(">")
	results, err := client.Do(ctx, cmd.Build()).AsXRead()
	if err != nil {
		// Check if error is due to no data available (nil response)
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error dequeueing task: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Get the first entry from the first stream
	for _, streamResult := range results {
		if len(streamResult) > 0 {
			return msgToTaskItem[T](streamResult[0])
		}
	}

	return nil, nil
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
	keyInfo := q.Keys(queue)

	// Get the task to find the ID
	msgs, err := client.Do(ctx, client.B().Xrange().Key(keyInfo.StreamKey).Start(taskID).End(taskID).Build()).AsXRange()
	if err != nil {
		// Check if error is due to no data available (nil response)
		if valkey.IsValkeyNil(err) {
			return nil
		}
		return fmt.Errorf("completing task: %w", err)
	}

	if len(msgs) == 0 {
		return nil
	}

	msg := msgs[0]
	id, ok := msg.FieldValues["id"]
	if !ok {
		return fmt.Errorf("completing task: missing id field")
	}

	// Remove from set
	err = client.Do(ctx, client.B().Srem().Key(keyInfo.SetKey).Member(id).Build()).Error()
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	// Acknowledge in consumer group
	err = client.Do(ctx, client.B().Xack().Key(keyInfo.StreamKey).Group(q.groupName).Id(taskID).Build()).Error()
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	// Delete from stream
	err = client.Do(ctx, client.B().Xdel().Key(keyInfo.StreamKey).Id(taskID).Build()).Error()
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) recover(ctx context.Context, client valkey.Client, queues []workflow.Queue, idleTimeout time.Duration) (*TaskItem[T], error) {
	for _, queue := range queues {
		streamKey := q.Keys(queue).StreamKey

		// Try to recover abandoned tasks
		cmd := client.B().Xautoclaim().Key(streamKey).Group(q.groupName).Consumer(q.workerName).MinIdleTime(strconv.FormatInt(idleTimeout.Milliseconds(), 10)).Start("0").Count(1)
		msgs, err := client.Do(ctx, cmd.Build()).ToArray()
		if err != nil {
			// Check if error is due to no data available (nil response)
			if valkey.IsValkeyNil(err) {
				continue
			}
			return nil, fmt.Errorf("recovering abandoned task: %w", err)
		}

		if len(msgs) >= 2 {
			entries, _ := msgs[1].ToArray()
			for _, entry := range entries {
				arr, _ := entry.ToArray()
				if len(arr) == 2 {
					id, _ := arr[0].ToString()
					fieldsArr, _ := arr[1].ToArray()
					fieldValues := map[string]string{}
					for i := 0; i+1 < len(fieldsArr); i += 2 {
						key, _ := fieldsArr[i].ToString()
						val, _ := fieldsArr[i+1].ToString()
						fieldValues[key] = val
					}
					xEntry := valkey.XRangeEntry{
						ID:          id,
						FieldValues: fieldValues,
					}
					return msgToTaskItem[T](xEntry)
				}
			}
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
