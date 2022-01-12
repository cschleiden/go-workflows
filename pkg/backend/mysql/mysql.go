package mysql

import (
	"context"
	"database/sql"
	_ "embed"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

func NewMysqlBackend(connection string) backend.Backend {
	db, err := sql.Open("mysql", connection)
	if err != nil {
		panic(err)
	}

	// TODO: Connect to database and migrate schema

	return &mysqlBackend{
		db: db,
	}
}

type mysqlBackend struct {
	db *sql.DB
}

// CreateWorkflowInstance creates a new workflow instance
func (b *mysqlBackend) CreateWorkflowInstance(_ context.Context, _ core.WorkflowEvent) error {
	panic("not implemented") // TODO: Implement
}

// SignalWorkflow signals a running workflow instance
func (b *mysqlBackend) SignalWorkflow(_ context.Context, _ core.WorkflowInstance, _ history.Event) error {
	panic("not implemented") // TODO: Implement
}

// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
func (b *mysqlBackend) GetWorkflowTask(_ context.Context) (*task.Workflow, error) {
	panic("not implemented") // TODO: Implement
}

// CompleteWorkflowTask completes a workflow task retrieved using GetWorkflowTask
//
// This checkpoints the execution. events are new events from the last workflow execution
// which will be added to the workflow instance history. workflowEvents are new events for the
// completed or other workflow instances.
func (b *mysqlBackend) CompleteWorkflowTask(ctx context.Context, task task.Workflow, events []history.Event, workflowEvents []core.WorkflowEvent) error {
	panic("not implemented") // TODO: Implement
}

// GetActivityTask returns a pending activity task or nil if there are no pending activities
func (b *mysqlBackend) GetActivityTask(_ context.Context) (*task.Activity, error) {
	panic("not implemented") // TODO: Implement
}

// CompleteActivityTask completes a activity task retrieved using GetActivityTask
func (b *mysqlBackend) CompleteActivityTask(_ context.Context, _ core.WorkflowInstance, _ string, _ history.Event) error {
	panic("not implemented") // TODO: Implement
}
