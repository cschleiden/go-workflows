package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/web"
)

var _ web.Backend = (*sqliteBackend)(nil)

func (sb *sqliteBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*web.WorkflowInstanceRef, error) {
	var err error
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if afterInstanceID != "" {
		rows, err = tx.QueryContext(
			ctx,
			`SELECT i.id, i.execution_id, i.created_at, i.completed_at
			FROM instances i
			INNER JOIN (SELECT id, created_at FROM instances WHERE id = ?) ii
				ON i.created_at < ii.created_at OR (i.created_at = ii.created_at AND i.id < ii.id)
			ORDER BY i.created_at DESC, i.id DESC
			LIMIT ?`,
			afterInstanceID,
			count,
		)
	} else {
		rows, err = tx.QueryContext(
			ctx,
			`SELECT i.id, i.execution_id, i.created_at, i.completed_at
			FROM instances i
			ORDER BY i.created_at DESC, i.id DESC
			LIMIT ?`,
			count,
		)
	}
	if err != nil {
		return nil, err
	}

	var instances []*web.WorkflowInstanceRef

	for rows.Next() {
		var id, executionID string
		var createdAt time.Time
		var completedAt *time.Time
		err = rows.Scan(&id, &executionID, &createdAt, &completedAt)
		if err != nil {
			return nil, err
		}

		var state backend.WorkflowState
		if completedAt != nil {
			state = backend.WorkflowStateFinished
		}

		instances = append(instances, &web.WorkflowInstanceRef{
			Instance:    core.NewWorkflowInstance(id, executionID),
			CreatedAt:   createdAt,
			CompletedAt: completedAt,
			State:       state,
		})
	}

	return instances, nil
}

func (sb *sqliteBackend) GetWorkflowInstance(ctx context.Context, instanceID string) (*web.WorkflowInstanceRef, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := tx.QueryRowContext(ctx, "SELECT id, execution_id, created_at, completed_at FROM instances WHERE id = ?", instanceID)

	var id, executionID string
	var createdAt time.Time
	var completedAt *time.Time

	err = res.Scan(&id, &executionID, &createdAt, &completedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	var state backend.WorkflowState
	if completedAt != nil {
		state = backend.WorkflowStateFinished
	}

	return &web.WorkflowInstanceRef{
		Instance:    core.NewWorkflowInstance(id, executionID),
		CreatedAt:   createdAt,
		CompletedAt: completedAt,
		State:       state,
	}, nil
}
