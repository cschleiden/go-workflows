package valkey

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/assert"
)

func Test_TaskQueue(t *testing.T) {
	// These tests rely on a Valkey server on localhost:6379.
	// Skip when running with -short.
	if testing.Short() {
		t.Skip()
	}

	taskType := "taskType"

	client := getClient()

	lockTimeout := time.Millisecond * 10
	blockTimeout := time.Millisecond * 10

	tests := []struct {
		name string
		f    func(t *testing.T, q *taskQueue[any])
	}{
		{
			name: "Simple enqueue/dequeue",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Size",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				s1, err := q.Size(ctx, client)
				assert.NoError(t, err)
				assert.Equal(t, map[workflow.Queue]int64{workflow.QueueDefault: 1}, s1)
			},
		},
		{
			name: "Guarantee uniqueness",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))
				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)

				assert.NoError(t, q.Complete(ctx, client, workflow.QueueDefault, task.TaskID))

				// After completion, the same id can be enqueued again
				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))
			},
		},
		{
			name: "Store custom data",
			f: func(t *testing.T, _ *taskQueue[any]) {
				type foo struct {
					Count int    `json:"count"`
					Name  string `json:"name"`
				}

				ctx := context.Background()

				q, err := newTaskQueue[foo]("prefix", taskType, "")
				assert.NoError(t, err)

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", &foo{
					Count: 1,
					Name:  "bar",
				}))

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, "t1", task.ID)
				assert.Equal(t, 1, task.Data.Count)
				assert.Equal(t, "bar", task.Data.Name)
			},
		},
		{
			name: "Simple enqueue/dequeue different worker",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				q2, err := newTaskQueue[any]("prefix", taskType, "")
				assert.NoError(t, err)

				// Dequeue using second worker
				task, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Complete removes task",
			f: func(t *testing.T, q *taskQueue[any]) {
				q2, err := newTaskQueue[any]("prefix", taskType, "")
				assert.NoError(t, err)

				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)

				// Complete task using second worker
				assert.NoError(t, q2.Complete(ctx, client, workflow.QueueDefault, task.TaskID))

				time.Sleep(time.Millisecond * 10)

				// Try to recover using second worker; should not find anything
				task2, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.Nil(t, task2)
			},
		},
		{
			name: "Recover task",
			f: func(t *testing.T, _ *taskQueue[any]) {
				type taskData struct {
					Count int `json:"count"`
				}
				q, err := newTaskQueue[taskData]("prefix", taskType, "")
				assert.NoError(t, err)

				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", &taskData{Count: 42}))

				q2, err := newTaskQueue[taskData]("prefix", taskType, "")
				assert.NoError(t, err)

				task, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, "t1", task.ID)

				time.Sleep(time.Millisecond * 10)

				// Assume q2 crashed, recover from other worker
				recoveredTask, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, time.Millisecond*1, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, recoveredTask)
				assert.Equal(t, task, recoveredTask)
			},
		},
		{
			name: "Extending task prevents recovering",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				// Create second worker (with different name)
				q2, err := newTaskQueue[any]("prefix", taskType, "")
				assert.NoError(t, err)

				task, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, "t1", task.ID)

				time.Sleep(time.Millisecond * 5)

				assert.NoError(t, q2.Extend(ctx, client, workflow.QueueDefault, task.TaskID))

				// Use large lock timeout; should not recover
				recoveredTask, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, time.Second*2, blockTimeout)
				assert.NoError(t, err)
				assert.Nil(t, recoveredTask)
			},
		},
		{
			name: "Will only dequeue from given queue",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				assert.NoError(t, q.Enqueue(ctx, client, workflow.QueueDefault, "t1", nil))

				assert.NoError(t, q.Prepare(ctx, client, []workflow.Queue{core.QueueSystem, workflow.QueueDefault}))

				task, err := q.Dequeue(ctx, client, []workflow.Queue{core.QueueSystem}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.Nil(t, task)

				task, err = q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				assert.NoError(t, err)
				assert.NotNil(t, task)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Best-effort cleanup between tests
			client.Do(ctx, client.B().Flushdb().Build())

			q, err := newTaskQueue[any]("prefix", taskType, "")
			assert.NoError(t, err)

			assert.NoError(t, q.Prepare(ctx, client, []workflow.Queue{workflow.QueueDefault}))

			tt.f(t, q)
		})
	}
}
