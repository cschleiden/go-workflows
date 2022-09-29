package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/core"
)

var _ diag.Backend = (*postgresBackend)(nil)

func (mb *postgresBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*diag.WorkflowInstanceRef, error) {
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
			FROM gwf.instances i
			INNER JOIN (SELECT instance_id, created_at FROM gwf.instances WHERE id = $1) ii
				ON i.created_at < ii.created_at OR (i.created_at = ii.created_at AND i.instance_id < ii.instance_id)
			ORDER BY i.created_at DESC, i.instance_id DESC
			LIMIT $2`,
			afterInstanceID,
			count,
		)
	} else {
		rows, err = tx.QueryContext(
			ctx,
			`SELECT i.instance_id, i.execution_id, i.created_at, i.completed_at
			FROM gwf.instances i
			ORDER BY i.created_at DESC, i.instance_id DESC
			LIMIT $1`,
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

		var state core.WorkflowInstanceState
		if completedAt != nil {
			state = core.WorkflowInstanceStateFinished
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

func (mb *postgresBackend) GetWorkflowInstance(ctx context.Context, instanceID string) (*diag.WorkflowInstanceRef, error) {
	tx, err := mb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := tx.QueryRowContext(ctx, "SELECT instance_id, execution_id, created_at, completed_at FROM gwf.instances WHERE instance_id = $1", instanceID)

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

	var state core.WorkflowInstanceState
	if completedAt != nil {
		state = core.WorkflowInstanceStateFinished
	}

	return &diag.WorkflowInstanceRef{
		Instance:    core.NewWorkflowInstance(id, executionID),
		CreatedAt:   createdAt,
		CompletedAt: completedAt,
		State:       state,
	}, nil
}
