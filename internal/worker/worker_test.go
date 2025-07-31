package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWorker(t *testing.T) {
	newStr := func(s string) *string {
		return &s
	}

	t.Run("heartbeat lengths", func(t *testing.T) {
		// For this test, we poll a task but wait some time before starting work.
		// We anticipate that two heartbeats will be sent; The first will
		// be sent early, relative to when work starts, due to the time spent waiting.
		// The second will be sent after a regular heartbeat interval.
		be := backend.NewMockBackend(t)
		logBuffer := &bytes.Buffer{}
		logger := slog.New(slog.NewTextHandler(logBuffer, nil))
		be.On("Options").Return(&backend.Options{Logger: logger}).Once()
		const heartbeatInterval = 1 * time.Second
		tw := NewMockTaskWorker[string, string](t)
		w := NewWorker(be, tw, &WorkerOptions{
			Pollers:           2,
			HeartbeatInterval: heartbeatInterval,
			MaxParallelTasks:  1,
			PollingInterval:   100 * time.Millisecond,
		})
		tw.On("Start", mock.Anything, mock.Anything).Return(nil).Once()
		t1 := newStr("task1")
		tw.On("Get", mock.Anything, mock.Anything).Return(t1, nil).Once()
		tw.On("Get", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		taskExtensions := []time.Time{}
		var mu sync.Mutex
		tw.On("Extend", mock.Anything, t1).Run(func(args mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()
			if ctx.Err() != nil {
				return
			}
			taskExtensions = append(taskExtensions, time.Now())
		}).Return(nil).Maybe()
		pollTime := time.Now()
		startWorkTime := pollTime.Add(heartbeatInterval / 2)
		callNum := 0
		w.currentTime = func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			callNum++
			switch callNum {
			case 1:
				return pollTime
			case 2:
				return startWorkTime
			default:
				panic("unexpected call to currentTime")
			}
		}
		r1 := newStr("result1")
		tw.On("Execute", mock.Anything, t1).Return(r1, nil).Once()
		done := make(chan struct{})
		tw.On("Complete", mock.Anything, r1, t1).Return(nil).Once().Run(func(args mock.Arguments) {
			time.Sleep(2 * time.Second)
			close(done)
		})
		require.NoError(t, w.Start(ctx))
		<-done
		func() {
			mu.Lock()
			defer mu.Unlock()
			cancel()
		}()
		require.NoError(t, w.WaitForCompletion())

		assert.Len(t, taskExtensions, 2)

		// first heartbeat
		timeBeforeFirstExtend := taskExtensions[0].Sub(pollTime).Nanoseconds()
		timeBetweenStartAndPoll := startWorkTime.Sub(pollTime).Nanoseconds()
		expectedTimeBeforeFirst := heartbeatInterval.Nanoseconds() - timeBetweenStartAndPoll
		wiggleRoom := float64(heartbeatInterval.Nanoseconds()) * 0.05 // 5% wiggle room
		assert.InDelta(t, expectedTimeBeforeFirst, timeBeforeFirstExtend, wiggleRoom)

		// second heartbeat
		timeBetweenFirstAndSecond := taskExtensions[1].Sub(taskExtensions[0]).Nanoseconds()
		expectedTimeBetweenFirstAndSecond := heartbeatInterval.Nanoseconds()
		assert.InDelta(t, expectedTimeBetweenFirstAndSecond, timeBetweenFirstAndSecond, wiggleRoom)
	})

	t.Run("a stopped heartbeat cancels task completion", func(t *testing.T) {
		// For this test, we poll a task and then return an error
		// on the first call to Extend. We make the Execute call
		// artificially long so that it gets interrupted by the cancellation
		// of the task concept, triggered by the stopped heartbeat.
		be := backend.NewMockBackend(t)
		logBuffer := &bytes.Buffer{}
		logger := slog.New(slog.NewTextHandler(logBuffer, nil))
		be.On("Options").Return(&backend.Options{Logger: logger}).Once()
		const heartbeatInterval = 1 * time.Second
		tw := NewMockTaskWorker[string, string](t)
		w := NewWorker(be, tw, &WorkerOptions{
			Pollers:           2,
			HeartbeatInterval: heartbeatInterval,
			MaxParallelTasks:  1,
			PollingInterval:   100 * time.Millisecond,
		})
		tw.On("Start", mock.Anything, mock.Anything).Return(nil).Once()
		t1 := newStr("task1")
		tw.On("Get", mock.Anything, mock.Anything).Return(t1, nil).Once()
		tw.On("Get", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		tw.On("Extend", mock.Anything, t1).Return(errors.New("test error")).Once()
		executeCompleted := false
		done := make(chan struct{})
		tw.On("Execute", mock.Anything, t1).Return(nil, context.Canceled).Once().Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			select {
			case <-time.After(2 * heartbeatInterval):
				executeCompleted = true
			case <-ctx.Done():
			}
			close(done)
		})
		require.NoError(t, w.Start(ctx))
		<-done
		cancel()
		require.NoError(t, w.WaitForCompletion())
		assert.False(t, executeCompleted)
	})
}
