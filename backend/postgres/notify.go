package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lib/pq" // Required for LISTEN/NOTIFY support
)

const (
	workflowTasksChannel = "workflow_tasks"
	activityTasksChannel = "activity_tasks"

	// Connection parameters for pq.Listener
	listenerMinReconnectInterval = 10 * time.Second
	listenerMaxReconnectInterval = time.Minute

	// Ping interval to keep LISTEN connection alive
	listenerPingInterval = 90 * time.Second
)

// notificationListener manages a single LISTEN/NOTIFY connection for reactive
// task polling. Both channels are multiplexed over one session so a process
// holds exactly one listener connection to Postgres.
type notificationListener struct {
	dsn    string
	logger *slog.Logger

	listener *pq.Listener

	workflowNotify chan struct{}
	activityNotify chan struct{}

	mu      sync.Mutex
	started bool
	closed  bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newNotificationListener(dsn string, logger *slog.Logger) *notificationListener {
	return &notificationListener{
		dsn:            dsn,
		logger:         logger,
		workflowNotify: make(chan struct{}, 1),
		activityNotify: make(chan struct{}, 1),
	}
}

// Start begins listening for notifications
func (nl *notificationListener) Start(ctx context.Context) error {
	nl.mu.Lock()
	defer nl.mu.Unlock()

	if nl.started {
		return nil
	}

	// Create a cancellable context for the handler goroutine
	nl.ctx, nl.cancel = context.WithCancel(context.Background())

	nl.listener = pq.NewListener(nl.dsn, listenerMinReconnectInterval, listenerMaxReconnectInterval, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			nl.logger.Error("notification listener event", "event", ev, "error", err)
		}
	})

	if err := nl.listener.Listen(workflowTasksChannel); err != nil {
		nl.listener.Close()
		return fmt.Errorf("listening to workflow tasks channel: %w", err)
	}

	if err := nl.listener.Listen(activityTasksChannel); err != nil {
		nl.listener.Close()
		return fmt.Errorf("listening to activity tasks channel: %w", err)
	}

	nl.started = true

	nl.wg.Add(1)
	go nl.handleNotifications()

	return nil
}

// Close stops the listener
func (nl *notificationListener) Close() error {
	nl.mu.Lock()
	if nl.closed {
		nl.mu.Unlock()
		return nil
	}
	nl.closed = true

	// Cancel the context to stop the handler goroutine
	if nl.cancel != nil {
		nl.cancel()
	}
	nl.mu.Unlock()

	// Wait for the handler goroutine to finish
	nl.wg.Wait()

	// Now safe to close the listener and channels
	var err error
	if nl.listener != nil {
		if cerr := nl.listener.Close(); cerr != nil {
			err = fmt.Errorf("closing notification listener: %w", cerr)
		}
	}

	close(nl.workflowNotify)
	close(nl.activityNotify)

	return err
}

// handleNotifications dispatches incoming notifications to the per-kind
// wake channels. Sends are non-blocking: the channels have capacity 1 and
// coalesce bursts - a pending wake-up already covers any number of tasks
// because the consumer re-queries until empty.
func (nl *notificationListener) handleNotifications() {
	defer nl.wg.Done()

	for {
		select {
		case <-nl.ctx.Done():
			return
		case notification, ok := <-nl.listener.Notify:
			if !ok {
				return
			}
			if notification == nil {
				// pq sends nil after a connection loss + re-establish:
				// notifications may have been missed while disconnected.
				// Conservatively wake both consumers so they re-check.
				nl.nudge(nl.workflowNotify)
				nl.nudge(nl.activityNotify)
				continue
			}
			switch notification.Channel {
			case workflowTasksChannel:
				nl.nudge(nl.workflowNotify)
			case activityTasksChannel:
				nl.nudge(nl.activityNotify)
			}
		case <-time.After(listenerPingInterval):
			// Periodic ping to keep connection alive
			if err := nl.listener.Ping(); err != nil {
				nl.logger.Error("notification listener ping failed", "error", err)
			}
		}
	}
}

func (nl *notificationListener) nudge(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
		// Channel already has a pending notification
	}
}

// WaitForWorkflowTask waits for a workflow task notification or context end.
func (nl *notificationListener) WaitForWorkflowTask(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-nl.workflowNotify:
		return true
	}
}

// WaitForActivityTask waits for an activity task notification or context end.
func (nl *notificationListener) WaitForActivityTask(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-nl.activityNotify:
		return true
	}
}
