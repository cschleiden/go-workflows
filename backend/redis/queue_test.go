package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func Test_TaskQueue(t *testing.T) {
	// These tests rely on redis being running on localhost:6379. Skip this test if `-short` is set.
	if testing.Short() {
		t.Skip()
	}

	taskType := "taskType"

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{"localhost:6379"},
		Username: "",
		Password: "RedisPassw0rd",
		DB:       1,
	})

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

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Size",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				s1, err := q.Size(ctx, client)
				require.NoError(t, err)
				require.Equal(t, map[workflow.Queue]int64{workflow.QueueDefault: 1}, s1)
			},
		},
		{
			name: "Guarantee uniqueness",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				_, err = client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)

				_, err = client.Pipelined(ctx, func(p redis.Pipeliner) error {
					_, err := q.Complete(ctx, p, workflow.QueueDefault, task.TaskID)
					return err
				})
				require.NoError(t, err)

				_, err = client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)
			},
		},
		{
			name: "Store custom data",
			f: func(t *testing.T, _ *taskQueue[any]) {
				type foo struct {
					Count int
					Name  string
				}

				ctx := context.Background()

				q, err := newTaskQueue[foo](context.Background(), client, "prefix", taskType, "")
				require.NoError(t, err)

				_, err = client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", &foo{
						Count: 1,
						Name:  "bar",
					})
				})
				require.NoError(t, err)

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
				require.Equal(t, 1, task.Data.Count)
				require.Equal(t, "bar", task.Data.Name)
			},
		},
		{
			name: "Simple enqueue/dequeue different worker",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				q2, _ := newTaskQueue[any](context.Background(), client, "prefix", taskType, "")
				require.NoError(t, err)

				// Dequeue using second worker
				task, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Complete removes task",
			f: func(t *testing.T, q *taskQueue[any]) {
				q2, _ := newTaskQueue[any](context.Background(), client, "prefix", taskType, "")

				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				task, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)

				// Complete task
				_, err = client.Pipelined(ctx, func(p redis.Pipeliner) error {
					_, err := q2.Complete(ctx, p, workflow.QueueDefault, task.TaskID)
					return err
				})
				require.NoError(t, err)

				time.Sleep(time.Millisecond * 10)

				// Try to recover using second worker
				task2, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.Nil(t, task2)
			},
		},
		{
			name: "Recover task",
			f: func(t *testing.T, _ *taskQueue[any]) {
				type taskData struct {
					Count int `json:"count"`
				}
				q, _ := newTaskQueue[taskData](context.Background(), client, "prefix", taskType, "")

				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", &taskData{
						Count: 42,
					})
				})
				require.NoError(t, err)

				q2, _ := newTaskQueue[taskData](context.Background(), client, "prefix", taskType, "")
				require.NoError(t, err)

				task, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)

				time.Sleep(time.Millisecond * 10)

				// Assume q2 crashed, recover from other worker
				recoveredTask, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, time.Millisecond*1, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, task, recoveredTask)
			},
		},
		{
			name: "Extending task prevents recovering",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				// Create second worker (with different name)
				q2, _ := newTaskQueue[any](context.Background(), client, "prefix", taskType, "")
				require.NoError(t, err)

				task, err := q2.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)

				time.Sleep(time.Millisecond * 5)

				_, err = client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q2.Extend(ctx, p, workflow.QueueDefault, task.TaskID)
				})
				require.NoError(t, err)

				// Use large lock timeout
				recoveredTask, err := q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, time.Second*2, blockTimeout)
				require.NoError(t, err)
				require.Nil(t, recoveredTask)
			},
		},
		{
			name: "Will only dequeue from given queue",
			f: func(t *testing.T, q *taskQueue[any]) {
				ctx := context.Background()

				_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
					return q.Enqueue(ctx, p, workflow.QueueDefault, "t1", nil)
				})
				require.NoError(t, err)

				require.NoError(t, q.Prepare(ctx, client, []workflow.Queue{core.QueueSystem, workflow.QueueDefault}))

				task, err := q.Dequeue(ctx, client, []workflow.Queue{core.QueueSystem}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.Nil(t, task)

				task, err = q.Dequeue(ctx, client, []workflow.Queue{workflow.QueueDefault}, lockTimeout, blockTimeout)
				require.NoError(t, err)
				require.NotNil(t, task)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := client.FlushDB(context.Background()).Err(); err != nil {
				panic(err)
			}

			r, err := client.Keys(context.Background(), "*").Result()
			if err != nil {
				panic(err)
			}

			if len(r) > 0 {
				panic("Keys should've been empty" + strings.Join(r, ", "))
			}

			ctx := context.Background()

			q, err := newTaskQueue[any](ctx, client, "prefix", taskType, "")
			require.NoError(t, err)

			require.NoError(t, q.Prepare(ctx, client, []workflow.Queue{workflow.QueueDefault}))

			tt.f(t, q)
		})
	}
}
