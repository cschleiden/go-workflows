package sqlite

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/core"
)

func (sb *sqliteBackend) GetWorkflowInstances(ctx context.Context) ([]*core.WorkflowInstance, error) {
	tx, err := sb.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, "SELECT id, execution_id FROM instances")
	if err != nil {
		return nil, err
	}

	var instances []*core.WorkflowInstance

	for rows.Next() {
		var id, executionID string
		err = rows.Scan(&id, &executionID)
		if err != nil {
			return nil, err
		}

		instances = append(instances, core.NewWorkflowInstance(id, executionID))
	}

	return instances, nil
}
