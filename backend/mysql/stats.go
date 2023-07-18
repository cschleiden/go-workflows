package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
)

func (b *mysqlBackend) GetStats(ctx context.Context) (*backend.Stats, error) {
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

	// Get pending activities
	row = tx.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM activities")
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to query active activities: %w", err)
	}

	var pendingActivities int64
	if err := row.Scan(&pendingActivities); err != nil {
		return nil, fmt.Errorf("failed to scan active activities: %w", err)
	}

	s.PendingActivities = pendingActivities

	return s, nil
}
