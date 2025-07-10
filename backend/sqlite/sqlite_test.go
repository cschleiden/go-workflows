package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func Test_SqliteBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(WithBackendOptions(append(options, backend.WithStickyTimeout(0))...))
		// return NewSqliteBackend("test.sqlite", WithBackendOptions(append(options, backend.WithStickyTimeout(0))...))
	}, func(b test.TestBackend) {
		// Ensure we close the database so the next test will get a clean in-memory db
		require.NoError(t, b.(*sqliteBackend).Close())
	})
}

func Test_EndToEndSqliteBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(WithBackendOptions(append(options, backend.WithStickyTimeout(0))...))
	}, func(b test.TestBackend) {
		// Ensure we close the database so the next test will get a clean in-memory db
		require.NoError(t, b.Close())
	})
}

func Test_SqliteBackend_WorkerName(t *testing.T) {
	t.Run("DefaultWorkerName", func(t *testing.T) {
		backend := NewInMemoryBackend()
		defer backend.Close()

		// The default worker name should be in the format "worker-<uuid>"
		require.Contains(t, backend.workerName, "worker-")
		require.Len(t, backend.workerName, 43) // "worker-" (7) + UUID (36)
	})

	t.Run("CustomWorkerName", func(t *testing.T) {
		customWorkerName := "test-worker-123"
		backend := NewInMemoryBackend(WithWorkerName(customWorkerName))
		defer backend.Close()

		require.Equal(t, customWorkerName, backend.workerName)
	})

	t.Run("EmptyWorkerNameUsesDefault", func(t *testing.T) {
		backend := NewInMemoryBackend(WithWorkerName(""))
		defer backend.Close()

		// Empty worker name should fall back to UUID generation
		require.Contains(t, backend.workerName, "worker-")
		require.Len(t, backend.workerName, 43) // "worker-" (7) + UUID (36)
	})

	t.Run("CustomWorkerNameIsUsedInDatabase", func(t *testing.T) {
		customWorkerName := "integration-test-worker"
		backend := NewInMemoryBackend(WithWorkerName(customWorkerName))
		defer backend.Close()

		// Verify the worker name is stored correctly
		require.Equal(t, customWorkerName, backend.workerName)

		// Create a workflow instance and task to ensure the worker name is actually used
		ctx := context.Background()
		instance := core.NewWorkflowInstance("test-instance", "test-execution")
		
		event := history.NewPendingEvent(
			time.Now(),
			history.EventType_WorkflowExecutionStarted,
			&history.ExecutionStartedAttributes{
				Queue: "test-queue",
				Metadata: &workflow.Metadata{},
			},
		)

		// Create workflow instance
		err := backend.CreateWorkflowInstance(ctx, instance, event)
		require.NoError(t, err)

		// Get a workflow task (this should lock it with our custom worker name)
		task, err := backend.GetWorkflowTask(ctx, []workflow.Queue{"test-queue"})
		require.NoError(t, err)
		require.NotNil(t, task)

		// Query the database to verify our custom worker name is used
		rows, err := backend.db.Query("SELECT worker FROM instances WHERE id = ? AND execution_id = ?", 
			instance.InstanceID, instance.ExecutionID)
		require.NoError(t, err)
		defer rows.Close()

		var workerNameFromDB string
		require.True(t, rows.Next())
		err = rows.Scan(&workerNameFromDB)
		require.NoError(t, err)
		require.Equal(t, customWorkerName, workerNameFromDB)
	})
}
