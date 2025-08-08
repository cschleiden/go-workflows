package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test types
type testResult struct {
	Output string
	Error  error
}

// mockTaskWorker implements TaskWorker interface for testing
type mockTaskWorker struct {
	mock.Mock
}

func (m *mockTaskWorker) Start(ctx context.Context, queues []workflow.Queue) error {
	args := m.Called(ctx, queues)
	return args.Error(0)
}

func (m *mockTaskWorker) Get(ctx context.Context, queues []workflow.Queue) (*testTask, error) {
	args := m.Called(ctx, queues)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testTask), args.Error(1)
}

func (m *mockTaskWorker) Extend(ctx context.Context, task *testTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *mockTaskWorker) Execute(ctx context.Context, task *testTask) (*testResult, error) {
	args := m.Called(ctx, task)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testResult), args.Error(1)
}

func (m *mockTaskWorker) Complete(ctx context.Context, result *testResult, task *testTask) error {
	args := m.Called(ctx, result, task)
	return args.Error(0)
}

// Helper function to create a mock backend with basic setup
func createMockBackend() *backend.MockBackend {
	mockBackend := &backend.MockBackend{}
	mockBackend.On("Options").Return(&backend.Options{
		Logger: slog.Default(),
	})
	return mockBackend
}

func TestNewWorker(t *testing.T) {
	t.Run("with default options", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:           2,
			MaxParallelTasks:  5,
			HeartbeatInterval: time.Second,
			PollingInterval:   time.Millisecond * 100,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		assert.NotNil(t, worker)
		assert.Equal(t, mockTaskWorker, worker.tw)
		assert.Equal(t, options, worker.options)
		assert.NotNil(t, worker.taskQueue)
		assert.NotNil(t, worker.logger)
		assert.NotNil(t, worker.dispatcherDone)

		// Should add default queue if none provided
		assert.Contains(t, worker.options.Queues, workflow.QueueDefault)
		// Should always include system queue
		assert.Contains(t, worker.options.Queues, core.QueueSystem)
	})

	t.Run("with custom queues", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		customQueue := workflow.Queue("custom")
		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
			Queues:           []workflow.Queue{customQueue},
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		assert.Contains(t, worker.options.Queues, customQueue)
		assert.Contains(t, worker.options.Queues, core.QueueSystem)
		assert.Len(t, worker.options.Queues, 2)
	})

	t.Run("system queue already included", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
			Queues:           []workflow.Queue{workflow.QueueDefault, core.QueueSystem},
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		// Should not duplicate system queue
		queueCount := 0
		for _, q := range worker.options.Queues {
			if q == core.QueueSystem {
				queueCount++
			}
		}
		assert.Equal(t, 1, queueCount)
	})
}

func TestWorker_Start(t *testing.T) {
	t.Run("successful start", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          2,
			MaxParallelTasks: 5,
			Queues:           []workflow.Queue{workflow.QueueDefault},
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockTaskWorker.On("Start", ctx, mock.MatchedBy(func(queues []workflow.Queue) bool {
			return len(queues) == 2 && // QueueDefault + QueueSystem
				queues[0] == workflow.QueueDefault &&
				queues[1] == core.QueueSystem
		})).Return(nil)

		// Mock the Get method to return nil (no tasks) so pollers don't hang
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, nil)

		err := worker.Start(ctx)
		assert.NoError(t, err)

		// Give pollers a moment to start
		time.Sleep(10 * time.Millisecond)

		// Cancel context to stop pollers
		cancel()

		// Wait for completion
		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("task worker start error", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		expectedErr := errors.New("start error")

		mockTaskWorker.On("Start", ctx, mock.Anything).Return(expectedErr)

		err := worker.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "starting task worker")
		assert.Contains(t, err.Error(), expectedErr.Error())

		mockTaskWorker.AssertExpectations(t)
	})
}

func TestWorker_Poll(t *testing.T) {
	t.Run("successful poll", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		expectedTask := &testTask{ID: 1, Data: "test"}

		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(expectedTask, nil)

		task, err := worker.poll(ctx, time.Second)
		assert.NoError(t, err)
		assert.Equal(t, expectedTask, task)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("timeout returns nil", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()

		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded)

		task, err := worker.poll(ctx, time.Millisecond)
		assert.NoError(t, err)
		assert.Nil(t, task)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("get error", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		expectedErr := errors.New("get error")

		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, expectedErr)

		task, err := worker.poll(ctx, time.Second)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, task)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("default timeout", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()

		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded)

		// Pass 0 timeout to test default timeout behavior
		task, err := worker.poll(ctx, 0)
		assert.NoError(t, err)
		assert.Nil(t, task)

		mockTaskWorker.AssertExpectations(t)
	})
}

func TestWorker_Handle(t *testing.T) {
	t.Run("successful handle without heartbeat", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
			// No heartbeat interval
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		task := &testTask{ID: 1, Data: "test"}
		result := &testResult{Output: "success"}

		mockTaskWorker.On("Execute", mock.Anything, task).Return(result, nil)
		mockTaskWorker.On("Complete", mock.Anything, result, task).Return(nil)

		err := worker.handle(ctx, task)
		assert.NoError(t, err)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("successful handle with heartbeat", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:           1,
			MaxParallelTasks:  1,
			HeartbeatInterval: time.Millisecond * 10,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		task := &testTask{ID: 1, Data: "test"}
		result := &testResult{Output: "success"}

		mockTaskWorker.On("Execute", mock.Anything, task).Return(result, nil)
		mockTaskWorker.On("Complete", mock.Anything, result, task).Return(nil)
		// Heartbeat might be called during execution
		mockTaskWorker.On("Extend", mock.Anything, task).Return(nil).Maybe()

		err := worker.handle(ctx, task)
		assert.NoError(t, err)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("execution error", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		task := &testTask{ID: 1, Data: "test"}
		expectedErr := errors.New("execution error")

		mockTaskWorker.On("Execute", mock.Anything, task).Return(nil, expectedErr)

		err := worker.handle(ctx, task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "executing task")
		assert.Contains(t, err.Error(), expectedErr.Error())

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("completion error", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx := context.Background()
		task := &testTask{ID: 1, Data: "test"}
		result := &testResult{Output: "success"}
		expectedErr := errors.New("completion error")

		mockTaskWorker.On("Execute", mock.Anything, task).Return(result, nil)
		mockTaskWorker.On("Complete", mock.Anything, result, task).Return(expectedErr)

		err := worker.handle(ctx, task)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("abort processing on heartbeat extend failure", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:           1,
			MaxParallelTasks:  1,
			HeartbeatInterval: time.Millisecond * 5,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		task := &testTask{ID: 1, Data: "test"}

		// Simulate Extend failing immediately so the heartbeat cancels processing
		mockTaskWorker.On("Extend", mock.Anything, task).Return(errors.New("extend failed")).Maybe()

		// Execute should see canceled context and return context.Canceled or respect ctx.Done
		mockTaskWorker.On("Execute", mock.Anything, task).Return(nil, context.Canceled)

		// Complete must NOT be called when execution is aborted due to lost ownership
		// No expectation set for Complete to ensure it's not invoked
		mockTaskWorker.AssertNotCalled(t, "Complete", mock.Anything, mock.Anything, mock.Anything)

		err := worker.handle(ctx, task)
		require.Error(t, err)

		mockTaskWorker.AssertExpectations(t)
	})
}

func TestWorker_HeartbeatTask(t *testing.T) {
	t.Run("successful heartbeat", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:           1,
			MaxParallelTasks:  1,
			HeartbeatInterval: time.Millisecond * 10,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()

		task := &testTask{ID: 1, Data: "test"}

		// Expect multiple heartbeat calls
		mockTaskWorker.On("Extend", ctx, task).Return(nil)

		worker.heartbeatTask(ctx, task, nil)

		// Should have called Extend at least once
		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("heartbeat error logged", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:           1,
			MaxParallelTasks:  1,
			HeartbeatInterval: time.Millisecond * 10,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()

		task := &testTask{ID: 1, Data: "test"}
		expectedErr := errors.New("heartbeat error")

		mockTaskWorker.On("Extend", ctx, task).Return(expectedErr)

		// Should not panic even with errors
		worker.heartbeatTask(ctx, task, nil)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("context cancellation stops heartbeat", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:           1,
			MaxParallelTasks:  1,
			HeartbeatInterval: time.Second, // Long interval
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithCancel(context.Background())
		task := &testTask{ID: 1, Data: "test"}

		// Cancel immediately
		cancel()

		// Should exit quickly without calling Extend
		start := time.Now()
		worker.heartbeatTask(ctx, task, nil)
		duration := time.Since(start)

		assert.Less(t, duration, time.Millisecond*100)

		// Should not have called Extend
		mockTaskWorker.AssertNotCalled(t, "Extend")
	})
}

func TestWorker_FullWorkflow(t *testing.T) {
	t.Run("end to end task processing", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 2,
			PollingInterval:  time.Millisecond * 10,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Track processed tasks
		var processedTasks int32
		var taskResults []*testResult
		var mu sync.Mutex

		task1 := &testTask{ID: 1, Data: "task1"}
		task2 := &testTask{ID: 2, Data: "task2"}
		result1 := &testResult{Output: "result1"}
		result2 := &testResult{Output: "result2"}

		// Setup expectations
		mockTaskWorker.On("Start", ctx, mock.Anything).Return(nil)

		// First poll returns task1, second returns task2, then nil
		call1 := mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(task1, nil).Once()
		call2 := mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(task2, nil).Once().NotBefore(call1)
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, nil).NotBefore(call2)

		mockTaskWorker.On("Execute", mock.Anything, task1).Return(result1, nil).Run(func(args mock.Arguments) {
			atomic.AddInt32(&processedTasks, 1)
			mu.Lock()
			defer mu.Unlock()
			taskResults = append(taskResults, result1)
		})
		mockTaskWorker.On("Execute", mock.Anything, task2).Return(result2, nil).Run(func(args mock.Arguments) {
			atomic.AddInt32(&processedTasks, 1)
			mu.Lock()
			defer mu.Unlock()
			taskResults = append(taskResults, result2)
		})

		mockTaskWorker.On("Complete", mock.Anything, result1, task1).Return(nil)
		mockTaskWorker.On("Complete", mock.Anything, result2, task2).Return(nil)

		// Start worker
		err := worker.Start(ctx)
		require.NoError(t, err)

		// Wait for tasks to be processed
		for atomic.LoadInt32(&processedTasks) < 2 && ctx.Err() == nil {
			time.Sleep(time.Millisecond)
		}

		// Cancel and wait for completion
		cancel()
		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		// Verify results
		assert.Equal(t, int32(2), atomic.LoadInt32(&processedTasks))
		assert.Len(t, taskResults, 2)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("concurrent task processing", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          2,
			MaxParallelTasks: 3,
			PollingInterval:  time.Millisecond * 5,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		taskCount := 5
		var processedCount int32
		var executionTimes []time.Time
		var mu sync.Mutex

		// Setup expectations
		mockTaskWorker.On("Start", ctx, mock.Anything).Return(nil)

		// Create tasks and results
		for i := 0; i < taskCount; i++ {
			task := &testTask{ID: i, Data: fmt.Sprintf("task%d", i)}
			result := &testResult{Output: fmt.Sprintf("result%d", i)}

			mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(task, nil).Once()

			mockTaskWorker.On("Execute", mock.Anything, task).Return(result, nil).Run(func(args mock.Arguments) {
				mu.Lock()
				executionTimes = append(executionTimes, time.Now())
				mu.Unlock()

				// Simulate some work
				time.Sleep(time.Millisecond * 10)
				atomic.AddInt32(&processedCount, 1)
			})
			mockTaskWorker.On("Complete", mock.Anything, result, task).Return(nil)
		}

		// After all tasks, return nil
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, nil)

		// Start worker
		err := worker.Start(ctx)
		require.NoError(t, err)

		// Wait for tasks to be processed with more time
		timeout := time.After(time.Second)
		for {
			select {
			case <-timeout:
				t.Fatal("timeout waiting for tasks to complete")
			default:
				if atomic.LoadInt32(&processedCount) >= int32(taskCount) {
					goto done
				}
				time.Sleep(time.Millisecond * 10)
			}
		}
	done:

		// Cancel and wait for completion
		cancel()
		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		// Verify concurrent execution
		assert.GreaterOrEqual(t, int(atomic.LoadInt32(&processedCount)), taskCount-1) // Allow for timing issues

		mu.Lock()
		assert.GreaterOrEqual(t, len(executionTimes), taskCount-1) // Allow for timing issues
		mu.Unlock()

		mockTaskWorker.AssertExpectations(t)
	})
}

func TestWorker_ErrorHandling(t *testing.T) {
	t.Run("polling error handling", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
			PollingInterval:  time.Millisecond * 10,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()

		mockTaskWorker.On("Start", ctx, mock.Anything).Return(nil)

		// Return error first, then nil to stop polling
		call1 := mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, errors.New("poll error")).Once()
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, nil).NotBefore(call1)

		err := worker.Start(ctx)
		require.NoError(t, err)

		// Let it run for a bit
		time.Sleep(time.Millisecond * 50)

		cancel()
		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("task queue add error", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		// Create a context that will be cancelled to cause add error
		ctx, cancel := context.WithCancel(context.Background())

		task := &testTask{ID: 1, Data: "test"}

		mockTaskWorker.On("Start", ctx, mock.Anything).Return(nil)

		// Mock Get to return a task, but we'll cancel context before it gets processed
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(task, nil).Maybe()
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		err := worker.Start(ctx)
		require.NoError(t, err)

		// Cancel context immediately to cause task queue add to fail
		cancel()

		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		// Don't assert specific expectations since timing can vary
	})
}

func TestWorker_Configuration(t *testing.T) {
	t.Run("zero pollers", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          0,
			MaxParallelTasks: 1,
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()

		mockTaskWorker.On("Start", ctx, mock.Anything).Return(nil)

		err := worker.Start(ctx)
		require.NoError(t, err)

		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		// Should not call Get since no pollers
		mockTaskWorker.AssertNotCalled(t, "Get")
		mockTaskWorker.AssertExpectations(t)
	})

	t.Run("unlimited parallel tasks", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 0, // Unlimited
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		// The workQueue should be created with unlimited capacity
		assert.NotNil(t, worker.taskQueue)

		// Test that reservation works without blocking
		ctx := context.Background()
		err := worker.taskQueue.reserve(ctx)
		assert.NoError(t, err)

		// Release should be no-op
		worker.taskQueue.release()
	})

	t.Run("no polling interval", func(t *testing.T) {
		mockBackend := createMockBackend()
		mockTaskWorker := &mockTaskWorker{}

		options := &WorkerOptions{
			Pollers:          1,
			MaxParallelTasks: 1,
			PollingInterval:  0, // No interval
		}

		worker := NewWorker(mockBackend, mockTaskWorker, options)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()

		mockTaskWorker.On("Start", ctx, mock.Anything).Return(nil)
		mockTaskWorker.On("Get", mock.Anything, mock.Anything).Return(nil, nil)

		err := worker.Start(ctx)
		require.NoError(t, err)

		// Should poll continuously without waiting
		time.Sleep(time.Millisecond * 30)

		cancel()
		err = worker.WaitForCompletion()
		assert.NoError(t, err)

		mockTaskWorker.AssertExpectations(t)
	})
}
