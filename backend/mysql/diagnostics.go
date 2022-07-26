package mysql

import (
	"context"
	"database/sql"
	"time"

	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/diag"
	"github.com/ticctech/go-workflows/internal/core"
)

var _ diag.Backend = (*mysqlBackend)(nil)

func (mb *mysqlBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*diag.WorkflowInstanceRef, error) {
	var err error
	tx, err := mb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if afterInstanceID != "" {
		rows, err = tx.QueryContext(
			ctx,
			`SELECT i.instance_id, i.execution_id, i.created_at, i.completed_at
			FROM instances i
			INNER JOIN (SELECT instance_id, created_at FROM instances WHERE id = ?) ii
				ON i.created_at < ii.created_at OR (i.created_at = ii.created_at AND i.instance_id < ii.instance_id)
			ORDER BY i.created_at DESC, i.instance_id DESC
			LIMIT ?`,
			afterInstanceID,
			count,
		)
	} else {
		rows, err = tx.QueryContext(
			ctx,
			`SELECT i.instance_id, i.execution_id, i.created_at, i.completed_at
			FROM instances i
			ORDER BY i.created_at DESC, i.instance_id DESC
			LIMIT ?`,
			count,
		)
	}
	if err != nil {
		return nil, err
	}

	var instances []*diag.WorkflowInstanceRef

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

		instances = append(instances, &diag.WorkflowInstanceRef{
			Instance:    core.NewWorkflowInstance(id, executionID),
			CreatedAt:   createdAt,
			CompletedAt: completedAt,
			State:       state,
		})
	}

	return instances, nil
}

func (mb *mysqlBackend) GetWorkflowInstance(ctx context.Context, instanceID string) (*diag.WorkflowInstanceRef, error) {
	tx, err := mb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := tx.QueryRowContext(ctx, "SELECT instance_id, execution_id, created_at, completed_at FROM instances WHERE instance_id = ?", instanceID)

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

	return &diag.WorkflowInstanceRef{
		Instance:    core.NewWorkflowInstance(id, executionID),
		CreatedAt:   createdAt,
		CompletedAt: completedAt,
		State:       state,
	}, nil
}
