package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
)

func (mb *mysqlBackend) GetStats(ctx context.Context) (*backend.Stats, error) {
	s := &backend.Stats{}

	tx, err := mb.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get active instances
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

	// Get workflow instances ready to be picked up
	now := time.Now()
	workflowRows, err := tx.QueryContext(
		ctx,
		`SELECT i.queue, COUNT(*)
			FROM instances i
			INNER JOIN pending_events pe ON i.instance_id = pe.instance_id
			WHERE
				i.state = ? AND i.completed_at IS NULL
				AND (pe.visible_at IS NULL OR pe.visible_at <= ?)
				AND (i.locked_until IS NULL OR i.locked_until < ?)
			GROUP BY i.queue`,
		core.WorkflowInstanceStateActive,
		now, // event.visible_at
		now, // locked_until
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	s.PendingWorkflowTasks = make(map[core.Queue]int64)

	for workflowRows.Next() {
		var queue string
		var pendingInstances int64
		if err := workflowRows.Scan(&queue, &pendingInstances); err != nil {
			return nil, fmt.Errorf("failed to scan active instances: %w", err)
		}

		s.PendingWorkflowTasks[workflow.Queue(queue)] = pendingInstances
	}

	if err := workflowRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read active instances: %w", err)
	}

	// Get pending activities
	activityRows, err := tx.QueryContext(
		ctx,
		"SELECT queue, COUNT(*) FROM activities GROUP BY queue")
	if err != nil {
		return nil, fmt.Errorf("failed to query active activities: %w", err)
	}

	s.PendingActivityTasks = make(map[core.Queue]int64)

	for activityRows.Next() {
		var queue string
		var pendingActivities int64
		if err := activityRows.Scan(&queue, &pendingActivities); err != nil {
			return nil, fmt.Errorf("failed to scan active activities: %w", err)
		}

		s.PendingActivityTasks[workflow.Queue(queue)] = pendingActivities
	}

	if err := activityRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read active activities: %w", err)
	}

	return s, nil
}
