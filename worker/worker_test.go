package worker

import (
	"testing"

	"github.com/cschleiden/go-workflows/backend/memory"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/registry"
	"github.com/stretchr/testify/require"
)

func worker_workflow_v1(ctx sync.Context) error {
	return nil
}

func worker_workflow_v2(ctx sync.Context) (string, error) {
	return "v2", nil
}

func TestWorker_RegisterWorkflowVersion(t *testing.T) {
	backend := memory.NewBackend()
	worker := New(backend, nil)

	// Test successful versioned workflow registration
	err := worker.RegisterWorkflowVersion(worker_workflow_v1, "1.0.0")
	require.NoError(t, err)

	// Test registering different version of same workflow
	err = worker.RegisterWorkflowVersion(worker_workflow_v2, "2.0.0")
	require.NoError(t, err)

	// Test registering same version again - should fail
	err = worker.RegisterWorkflowVersion(worker_workflow_v1, "1.0.0")
	var wantErr *registry.ErrVersionedWorkflowAlreadyRegistered
	require.ErrorAs(t, err, &wantErr)

	// Test with custom name
	err = worker.RegisterWorkflowVersion(worker_workflow_v1, "1.0.0", registry.WithName("CustomWorkflow"))
	require.NoError(t, err)
}

func TestWorker_RegisterWorkflowVersion_InvalidWorkflow(t *testing.T) {
	backend := memory.NewBackend()
	worker := New(backend, nil)

	// Test empty version
	err := worker.RegisterWorkflowVersion(worker_workflow_v1, "")
	require.Error(t, err)

	// Test invalid workflow
	err = worker.RegisterWorkflowVersion("not a function", "1.0.0")
	require.Error(t, err)

	// Test workflow without context
	err = worker.RegisterWorkflowVersion(func() error { return nil }, "1.0.0")
	require.Error(t, err)
}

func TestWorker_VersionedWorkflow_Integration(t *testing.T) {
	backend := memory.NewBackend()
	worker := New(backend, nil)

	// Register both regular and versioned workflows
	err := worker.RegisterWorkflow(worker_workflow_v1)
	require.NoError(t, err)

	err = worker.RegisterWorkflowVersion(worker_workflow_v1, "1.0.0")
	require.NoError(t, err)

	err = worker.RegisterWorkflowVersion(worker_workflow_v2, "2.0.0")
	require.NoError(t, err)

	// Verify they can be retrieved from the registry
	regularWorkflow, err := worker.registry.GetWorkflow("worker_workflow_v1")
	require.NoError(t, err)
	require.NotNil(t, regularWorkflow)

	versionedWorkflow1, err := worker.registry.GetVersionedWorkflow("worker_workflow_v1", "1.0.0")
	require.NoError(t, err)
	require.NotNil(t, versionedWorkflow1)

	versionedWorkflow2, err := worker.registry.GetVersionedWorkflow("worker_workflow_v2", "2.0.0")
	require.NoError(t, err)
	require.NotNil(t, versionedWorkflow2)
}