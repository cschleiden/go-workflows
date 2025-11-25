package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/valkey-io/valkey-glide/go/v2"
	"github.com/valkey-io/valkey-glide/go/v2/models"
	"github.com/valkey-io/valkey-glide/go/v2/options"
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

	return tq, nil
}

func (q *taskQueue[T]) Prepare(ctx context.Context, client glide.Client, queues []workflow.Queue) error {
	for _, queue := range queues {
		streamKey := q.Keys(queue).StreamKey
		if _, err := client.XGroupCreateWithOptions(ctx, streamKey, q.groupName, "0", options.XGroupCreateOptions{MkStream: true}); err != nil {
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
		StreamKey: fmt.Sprintf("%stask-stream:%s:%s", q.keyPrefix, queue, q.tasktype),
		SetKey:    fmt.Sprintf("%stask-set:%s:%s", q.keyPrefix, queue, q.tasktype),
	}
}

func (q *taskQueue[T]) Size(ctx context.Context, client glide.Client) (map[workflow.Queue]int64, error) {
	members, err := client.SMembers(ctx, q.queueSetKey)
	if err != nil {
		return nil, fmt.Errorf("getting queue size: %w", err)
	}

	res := map[workflow.Queue]int64{}
	for queueSetKey := range members {
		size, err := client.SCard(ctx, queueSetKey)
		if err != nil {
			return nil, fmt.Errorf("getting queue size: %w", err)
		}

		// Parse queue name from key
		queueName := strings.TrimPrefix(queueSetKey, q.keyPrefix)
		parts := strings.Split(queueName, ":") // task-set:<queue>:<tasktype>
		if len(parts) < 3 {
			return nil, fmt.Errorf("unexpected set key format: %s", queueSetKey)
		}
		queue := workflow.Queue(parts[1])
		res[queue] = size
	}

	return res, nil
}

func (q *taskQueue[T]) Enqueue(ctx context.Context, client glide.Client, queue workflow.Queue, id string, data *T) error {
	ds, err := json.Marshal(data)
	if err != nil {
		return err
	}

	keys := q.Keys(queue)

	// Add to set to track uniqueness
	_, err = client.SAdd(ctx, q.queueSetKey, []string{keys.SetKey})
	if err != nil {
		return err
	}

	// Add to set for this queue
	added, err := client.SAdd(ctx, keys.SetKey, []string{id})
	if err != nil {
		return err
	}

	// Only add to stream if it's a new task
	if added > 0 {
		var fieldValues []models.FieldValue
		fieldValues = append(fieldValues, models.FieldValue{Field: "id", Value: id})
		fieldValues = append(fieldValues, models.FieldValue{Field: "data", Value: string(ds)})

		_, err = client.XAdd(ctx, keys.StreamKey, fieldValues)
		if err != nil {
			return err
		}
	}

	return nil
}

func (q *taskQueue[T]) Dequeue(ctx context.Context, client glide.Client, queues []workflow.Queue, lockTimeout, timeout time.Duration) (*TaskItem[T], error) {
	// Try to recover abandoned tasks
	task, err := q.recover(ctx, client, queues, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("checking for abandoned tasks: %w", err)
	}

	if task != nil {
		return task, nil
	}

	// Check for new tasks
	keyAndIds := make(map[string]string)
	for _, queue := range queues {
		keyInfo := q.Keys(queue)
		keyAndIds[keyInfo.StreamKey] = ">"
	}

	// Try to dequeue from all given queues
	results, err := client.XReadGroupWithOptions(ctx, q.groupName, q.workerName, keyAndIds, options.XReadGroupOptions{
		Count: 1,
		Block: timeout,
	})

	if err != nil {
		return nil, fmt.Errorf("error dequeueing task: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Get the first entry to dequeue
	var entry models.StreamEntry
	for _, response := range results {
		if len(response.Entries) > 0 {
			entry = response.Entries[0]
			break
		}
	}

	return msgToTaskItem[T](entry)
}

func (q *taskQueue[T]) Extend(ctx context.Context, client glide.Client, queue workflow.Queue, taskID string) error {
	// Claiming a message resets the idle timer
	_, err := client.XClaim(ctx, q.Keys(queue).StreamKey, q.groupName, q.workerName, 0, []string{taskID})
	if err != nil {
		return fmt.Errorf("extending lease: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) Complete(ctx context.Context, client glide.Client, queue workflow.Queue, taskID string) error {
	keyInfo := q.Keys(queue)

	// Get the task to find the ID
	msgs, err := client.XRange(ctx, keyInfo.StreamKey, options.NewStreamBoundary(taskID, true), options.NewStreamBoundary(taskID, true))
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	if len(msgs) == 0 {
		return nil
	}

	msg := msgs[0]
	var id string
	for _, field := range msg.Fields {
		if field.Field == "id" {
			id = field.Value
		}
	}

	// Remove from set
	_, err = client.SRem(ctx, keyInfo.SetKey, []string{id})
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	// Acknowledge in consumer group
	_, err = client.XAck(ctx, keyInfo.StreamKey, q.groupName, []string{taskID})
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	// Delete from stream
	_, err = client.XDel(ctx, keyInfo.StreamKey, []string{taskID})
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}

	return nil
}

func (q *taskQueue[T]) recover(ctx context.Context, client glide.Client, queues []workflow.Queue, idleTimeout time.Duration) (*TaskItem[T], error) {
	for _, queue := range queues {
		streamKey := q.Keys(queue).StreamKey

		// Try to recover abandoned tasks
		msgs, err := client.XAutoClaimWithOptions(ctx, streamKey, q.groupName, q.workerName, idleTimeout, "0", options.XAutoClaimOptions{Count: 1})
		if err != nil {
			return nil, fmt.Errorf("recovering abandoned task: %w", err)
		}

		if len(msgs.ClaimedEntries) > 0 {
			return msgToTaskItem[T](msgs.ClaimedEntries[0])
		}
	}

	return nil, nil
}

func msgToTaskItem[T any](msg models.StreamEntry) (*TaskItem[T], error) {
	var id, data string
	for _, field := range msg.Fields {
		if field.Field == "id" {
			id = field.Value
		} else if field.Field == "data" {
			data = field.Value
		}
	}

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
