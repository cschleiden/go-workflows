package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
)

func (b *sqliteBackend) GetStats(ctx context.Context) (*backend.Stats, error) {
	s := &backend.Stats{}

	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM instances i WHERE i.completed_at IS NULL",
	)
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	var activeInstances int64
	if err := row.Scan(&activeInstances); err != nil {
		return nil, fmt.Errorf("failed to scan active instances: %w", err)
	}

	s.ActiveWorkflowInstances = activeInstances

	s.PendingTasksInQueue = map[core.Queue]*backend.QueueStats{}
	for _, q := range b.options.Queues {
		s.PendingTasksInQueue[q] = &backend.QueueStats{}
	}

	// Get workflow instances ready to be picked up
	now := time.Now()
	workflowRows, err := tx.QueryContext(
		ctx,
		`SELECT COUNT(*) FROM instances i
			WHERE
				(locked_until IS NULL OR locked_until < ?)
				AND state = ? AND i.completed_at IS NULL
				AND EXISTS (
					SELECT 1
						FROM pending_events
						WHERE instance_id = i.id AND execution_id = i.execution_id AND (visible_at IS NULL OR visible_at <= ?)
				)
			GROUP BY i.queue`,
		now,                              // locked_until
		core.WorkflowInstanceStateActive, // state
		now,                              // pending_event.visible_at
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	for workflowRows.Next() {
		var queue string
		var pendingInstances int64
		if err := workflowRows.Scan(&queue, &pendingInstances); err != nil {
			return nil, fmt.Errorf("failed to scan active instances: %w", err)
		}

		s.PendingTasksInQueue[core.Queue(queue)].PendingWorkflowTasks = pendingInstances
	}

	// Get pending activities
	activityRows, err := tx.QueryContext(
		ctx,
		"SELECT COUNT(*) FROM activities GRUOP BY queue")
	if err != nil {
		return nil, fmt.Errorf("failed to query active activities: %w", err)
	}

	for activityRows.Next() {
		var queue string
		var pendingActivities int64
		if err := activityRows.Scan(&queue, &pendingActivities); err != nil {
			return nil, fmt.Errorf("failed to scan active activities: %w", err)
		}

		s.PendingTasksInQueue[core.Queue(queue)].PendingActivities = pendingActivities
	}

	return s, nil
}
