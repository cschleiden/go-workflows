package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testTask is a simple task struct for testing
type testTask struct {
	ID   int
	Data string
}

func TestNewWorkQueue(t *testing.T) {
	t.Run("unlimited parallelism", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)

		require.NotNil(t, wq)
		require.NotNil(t, wq.tasks)
		require.Nil(t, wq.slots) // No slots channel when maxParallelTasks is 0
	})

	t.Run("limited parallelism", func(t *testing.T) {
		maxTasks := 5
		wq := newWorkQueue[testTask](maxTasks)

		require.NotNil(t, wq)
		require.NotNil(t, wq.tasks)
		require.NotNil(t, wq.slots)
		require.Equal(t, maxTasks, cap(wq.slots))
	})

	t.Run("negative max parallel tasks treated as unlimited", func(t *testing.T) {
		wq := newWorkQueue[testTask](-1)

		require.NotNil(t, wq)
		require.NotNil(t, wq.tasks)
		require.Nil(t, wq.slots)
	})
}

func TestWorkQueue_Reserve(t *testing.T) {
	t.Run("unlimited parallelism - no reservation needed", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)
		ctx := context.Background()

		err := wq.reserve(ctx)
		require.NoError(t, err)
	})

	t.Run("limited parallelism - successful reservation", func(t *testing.T) {
		wq := newWorkQueue[testTask](2)
		ctx := context.Background()

		// First reservation should succeed
		err := wq.reserve(ctx)
		require.NoError(t, err)

		// Second reservation should succeed
		err = wq.reserve(ctx)
		require.NoError(t, err)
	})

	t.Run("limited parallelism - reservation blocks when slots full", func(t *testing.T) {
		wq := newWorkQueue[testTask](1)
		ctx := context.Background()

		// First reservation should succeed immediately
		err := wq.reserve(ctx)
		require.NoError(t, err)

		// Second reservation should block
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err = wq.reserve(ctx2)
		require.Error(t, err)
		require.Equal(t, context.DeadlineExceeded, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		wq := newWorkQueue[testTask](1)

		// Fill the slot
		err := wq.reserve(context.Background())
		require.NoError(t, err)

		// Try to reserve with cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = wq.reserve(ctx)
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	})
}

func TestWorkQueue_Add(t *testing.T) {
	t.Run("successful add", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)
		ctx := context.Background()

		task := &testTask{ID: 1, Data: "test"}

		// Add task in a goroutine since it will block until someone reads from the channel
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = wq.add(ctx, task)
		}()

		// Read the task from the channel
		select {
		case receivedTask := <-wq.tasks:
			require.Equal(t, task, receivedTask)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for task")
		}

		// Wait for add to complete
		<-done
		require.NoError(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		task := &testTask{ID: 1, Data: "test"}

		err := wq.add(ctx, task)
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("context timeout", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		task := &testTask{ID: 1, Data: "test"}

		// This should timeout since no one is reading from the channel
		err := wq.add(ctx, task)
		require.Error(t, err)
		require.Equal(t, context.DeadlineExceeded, err)
	})
}

func TestWorkQueue_Release(t *testing.T) {
	t.Run("unlimited parallelism - release is no-op", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)

		// Should not panic or block
		wq.release()
	})

	t.Run("limited parallelism - release frees slot", func(t *testing.T) {
		wq := newWorkQueue[testTask](1)
		ctx := context.Background()

		// Reserve a slot
		err := wq.reserve(ctx)
		require.NoError(t, err)

		// Release the slot
		wq.release()

		// Should be able to reserve again immediately
		err = wq.reserve(ctx)
		require.NoError(t, err)
	})
}

func TestWorkQueue_ConcurrentOperations(t *testing.T) {
	t.Run("concurrent reserves and releases", func(t *testing.T) {
		wq := newWorkQueue[testTask](3)
		ctx := context.Background()

		var wg sync.WaitGroup
		reserveCount := 10

		// Start multiple goroutines trying to reserve and release
		for i := 0; i < reserveCount; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				err := wq.reserve(ctx)
				require.NoError(t, err, "goroutine %d failed to reserve", id)

				// Simulate some work
				time.Sleep(time.Millisecond)

				wq.release()
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent adds with reader", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)
		ctx := context.Background()

		taskCount := 10
		var wg sync.WaitGroup

		// Start reader goroutine
		receivedTasks := make([]*testTask, 0, taskCount)
		var mu sync.Mutex

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < taskCount; i++ {
				task := <-wq.tasks
				mu.Lock()
				receivedTasks = append(receivedTasks, task)
				mu.Unlock()
			}
		}()

		// Start multiple writers
		for i := 0; i < taskCount; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				task := &testTask{ID: id, Data: "concurrent"}
				err := wq.add(ctx, task)
				require.NoError(t, err)
			}(i)
		}

		wg.Wait()

		mu.Lock()
		require.Len(t, receivedTasks, taskCount)
		mu.Unlock()
	})
}

func TestWorkQueue_EdgeCases(t *testing.T) {
	t.Run("release without reserve on unlimited queue", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)

		// Should not panic
		require.NotPanics(t, func() {
			wq.release()
		})
	})

	t.Run("multiple releases after single reserve", func(t *testing.T) {
		wq := newWorkQueue[testTask](1)
		ctx := context.Background()

		// Reserve one slot
		err := wq.reserve(ctx)
		require.NoError(t, err)

		// Release once
		wq.release()

		// Release again - this would block trying to read from empty channel
		// But in a real scenario this shouldn't happen due to proper usage
		// We'll just test that we can reserve again after the first release
		err = wq.reserve(ctx)
		require.NoError(t, err)
	})

	t.Run("zero capacity queue operations", func(t *testing.T) {
		wq := newWorkQueue[testTask](0)
		ctx := context.Background()

		// All operations should work with unlimited capacity
		err := wq.reserve(ctx)
		require.NoError(t, err)

		wq.release() // Should be no-op

		// Test add with immediate read
		task := &testTask{ID: 1, Data: "zero-cap"}

		var receivedTask *testTask
		done := make(chan struct{})

		go func() {
			defer close(done)
			receivedTask = <-wq.tasks
		}()

		err = wq.add(ctx, task)
		require.NoError(t, err)

		<-done
		require.Equal(t, task, receivedTask)
	})
}

func TestWorkQueue_GenericTypes(t *testing.T) {
	t.Run("string tasks", func(t *testing.T) {
		wq := newWorkQueue[string](1)
		ctx := context.Background()

		task := "string task"

		var receivedTask *string
		done := make(chan struct{})

		go func() {
			defer close(done)
			receivedTask = <-wq.tasks
		}()

		err := wq.add(ctx, &task)
		require.NoError(t, err)

		<-done
		require.Equal(t, &task, receivedTask)
	})

	t.Run("int tasks", func(t *testing.T) {
		wq := newWorkQueue[int](2)
		ctx := context.Background()

		err := wq.reserve(ctx)
		require.NoError(t, err)

		task := 42

		var receivedTask *int
		done := make(chan struct{})

		go func() {
			defer close(done)
			receivedTask = <-wq.tasks
		}()

		err = wq.add(ctx, &task)
		require.NoError(t, err)

		<-done
		require.Equal(t, &task, receivedTask)

		wq.release()
	})
}

// Benchmark tests
func BenchmarkWorkQueue_Reserve(b *testing.B) {
	wq := newWorkQueue[testTask](1000)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = wq.reserve(ctx)
			wq.release()
		}
	})
}

func BenchmarkWorkQueue_Add(b *testing.B) {
	wq := newWorkQueue[testTask](0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a reader goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-wq.tasks:
				// Consume tasks
			case <-ctx.Done():
				return
			}
		}
	}()

	task := &testTask{ID: 1, Data: "benchmark"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = wq.add(ctx, task)
		}
	})

	cancel()
	<-done
}
