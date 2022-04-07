package taskqueue

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func Test_TaskQueue(t *testing.T) {
	// These cases rely on redis being running on localhost:6379. Skip this test if `-short` is set.
	if testing.Short() {
		t.Skip()
	}

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{"localhost:6379"},
		Username: "",
		Password: "RedisPassw0rd",
		DB:       1,
	})

	lockTimeout := time.Millisecond * 10

	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{
			name: "Create queue",
			f: func(t *testing.T) {
				q, err := New[any](client, "test")
				require.NoError(t, err)
				require.NotNil(t, q)
			},
		},
		{
			name: "Simple enqueue/dequeue",
			f: func(t *testing.T) {
				q, err := New[any](client, "test")
				require.NoError(t, err)

				_, err = q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)

				task, err := q.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Guarantee uniqueness",
			f: func(t *testing.T) {
				q, err := New[any](client, "test")
				require.NoError(t, err)

				_, err = q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)

				_, err = q.Enqueue(context.Background(), "t1", nil)
				require.Error(t, ErrTaskAlreadyInQueue, err)

				task, err := q.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)

				err = q.Complete(context.Background(), task.TaskID)
				require.NoError(t, err)

				_, err = q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)
			},
		},
		{
			name: "Store custom data",
			f: func(t *testing.T) {
				type foo struct {
					Count int
					Name  string
				}

				q, err := New[foo](client, "test")
				require.NoError(t, err)

				_, err = q.Enqueue(context.Background(), "t1", &foo{
					Count: 1,
					Name:  "bar",
				})
				require.NoError(t, err)

				task, err := q.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
				require.Equal(t, 1, task.Data.Count)
				require.Equal(t, "bar", task.Data.Name)
			},
		},
		{
			name: "Simple enqueue/dequeue different worker",
			f: func(t *testing.T) {
				q, _ := New[any](client, "test")

				_, err := q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)

				q2, _ := New[any](client, "test")
				require.NoError(t, err)

				// Dequeue using second worker
				task, err := q2.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Complete removes task",
			f: func(t *testing.T) {
				q, _ := New[any](client, "test")
				q2, _ := New[any](client, "test")

				_, err := q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)

				task, err := q.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)

				// Complete task
				err = q2.Complete(context.Background(), task.TaskID)
				require.NoError(t, err)

				time.Sleep(time.Millisecond * 10)

				// Try to recover using second worker
				task2, err := q2.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.Nil(t, task2)
			},
		},
		{
			name: "Recover task",
			f: func(t *testing.T) {
				q, _ := New[any](client, "test")

				_, err := q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)

				q2, _ := New[any](client, "test")
				require.NoError(t, err)

				task, err := q2.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)

				time.Sleep(time.Millisecond * 10)

				// Assume q2 crashed, recover from other worker
				recoveredTask, err := q.Dequeue(context.Background(), time.Millisecond*1, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, task, recoveredTask)
			},
		},
		{
			name: "Extending task prevents recovering",
			f: func(t *testing.T) {
				q, _ := New[any](client, "test")

				_, err := q.Enqueue(context.Background(), "t1", nil)
				require.NoError(t, err)

				q2, _ := New[any](client, "test")
				require.NoError(t, err)

				task, err := q2.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)

				time.Sleep(time.Millisecond * 10)

				err = q2.Extend(context.Background(), task.TaskID)
				require.NoError(t, err)

				recoveredTask, err := q.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.Nil(t, recoveredTask)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := client.FlushDB(context.Background()).Err(); err != nil {
				panic(err)
			}

			tt.f(t)
		})
	}
}
