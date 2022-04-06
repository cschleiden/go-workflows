package taskqueue

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func Test_TaskQueue(t *testing.T) {
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
				q, err := New(client, "test")
				require.NoError(t, err)
				require.NotNil(t, q)
			},
		},
		{
			name: "Simple enqueue/dequeue",
			f: func(t *testing.T) {
				q, err := New(client, "test")
				require.NoError(t, err)

				err = q.Enqueue(context.Background(), "t1")
				require.NoError(t, err)

				task, err := q.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Simple enqueue/dequeue different works",
			f: func(t *testing.T) {
				q, _ := New(client, "test")

				err := q.Enqueue(context.Background(), "t1")
				require.NoError(t, err)

				q2, _ := New(client, "test")
				require.NoError(t, err)

				// Dequeue using second worker
				task, err := q2.Dequeue(context.Background(), lockTimeout, time.Millisecond*10)
				require.NoError(t, err)
				require.NotNil(t, task)
				require.Equal(t, "t1", task.ID)
			},
		},
		{
			name: "Recover task",
			f: func(t *testing.T) {
				q, _ := New(client, "test")

				err := q.Enqueue(context.Background(), "t1")
				require.NoError(t, err)

				q2, _ := New(client, "test")
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
				q, _ := New(client, "test")

				err := q.Enqueue(context.Background(), "t1")
				require.NoError(t, err)

				q2, _ := New(client, "test")
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
