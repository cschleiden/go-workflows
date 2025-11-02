package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lib/pq"
)

const (
	workflowTasksChannel = "workflow_tasks"
	activityTasksChannel = "activity_tasks"
)

// notificationListener manages LISTEN/NOTIFY connections for reactive task polling
type notificationListener struct {
	dsn    string
	logger *slog.Logger

	workflowListener *pq.Listener
	activityListener *pq.Listener

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

	// Create a cancellable context for the handler goroutines
	nl.ctx, nl.cancel = context.WithCancel(context.Background())

	// Create listener for workflow tasks
	nl.workflowListener = pq.NewListener(nl.dsn, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			nl.logger.Error("workflow listener event", "event", ev, "error", err)
		}
	})

	if err := nl.workflowListener.Listen(workflowTasksChannel); err != nil {
		return fmt.Errorf("listening to workflow tasks channel: %w", err)
	}

	// Create listener for activity tasks
	nl.activityListener = pq.NewListener(nl.dsn, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			nl.logger.Error("activity listener event", "event", ev, "error", err)
		}
	})

	if err := nl.activityListener.Listen(activityTasksChannel); err != nil {
		nl.workflowListener.Close()
		return fmt.Errorf("listening to activity tasks channel: %w", err)
	}

	nl.started = true

	// Start goroutines to handle notifications
	nl.wg.Add(2)
	go nl.handleWorkflowNotifications()
	go nl.handleActivityNotifications()

	return nil
}

// Close stops the listeners
func (nl *notificationListener) Close() error {
	nl.mu.Lock()
	if nl.closed {
		nl.mu.Unlock()
		return nil
	}
	nl.closed = true

	// Cancel the context to stop handler goroutines
	if nl.cancel != nil {
		nl.cancel()
	}
	nl.mu.Unlock()

	// Wait for handler goroutines to finish
	nl.wg.Wait()

	// Now safe to close listeners and channels
	var errs []error
	if nl.workflowListener != nil {
		if err := nl.workflowListener.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing workflow listener: %w", err))
		}
	}

	if nl.activityListener != nil {
		if err := nl.activityListener.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing activity listener: %w", err))
		}
	}

	close(nl.workflowNotify)
	close(nl.activityNotify)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing listeners: %v", errs)
	}

	return nil
}

func (nl *notificationListener) handleWorkflowNotifications() {
	defer nl.wg.Done()

	for {
		select {
		case <-nl.ctx.Done():
			return
		case notification, ok := <-nl.workflowListener.Notify:
			if !ok {
				return
			}
			if notification != nil {
				// Non-blocking send to notify channel
				select {
				case nl.workflowNotify <- struct{}{}:
				default:
					// Channel already has a pending notification
				}
			}
		case <-time.After(90 * time.Second):
			// Periodic ping to keep connection alive
			if err := nl.workflowListener.Ping(); err != nil {
				nl.logger.Error("workflow listener ping failed", "error", err)
			}
		}
	}
}

func (nl *notificationListener) handleActivityNotifications() {
	defer nl.wg.Done()

	for {
		select {
		case <-nl.ctx.Done():
			return
		case notification, ok := <-nl.activityListener.Notify:
			if !ok {
				return
			}
			if notification != nil {
				// Non-blocking send to notify channel
				select {
				case nl.activityNotify <- struct{}{}:
				default:
					// Channel already has a pending notification
				}
			}
		case <-time.After(90 * time.Second):
			// Periodic ping to keep connection alive
			if err := nl.activityListener.Ping(); err != nil {
				nl.logger.Error("activity listener ping failed", "error", err)
			}
		}
	}
}

// WaitForWorkflowTask waits for a workflow task notification or timeout
func (nl *notificationListener) WaitForWorkflowTask(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-nl.workflowNotify:
		return true
	}
}

// WaitForActivityTask waits for an activity task notification or timeout
func (nl *notificationListener) WaitForActivityTask(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-nl.activityNotify:
		return true
	}
}
