package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifications_WorkflowTask(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Create test database
	adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
	require.NoError(t, err)
	defer adminDB.Close()

	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)

	defer func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
	}()

	// Create backend with notifications enabled
	b := NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName,
		WithNotifications(true),
		WithBackendOptions(backend.WithStickyTimeout(0)))
	defer b.Close()

	ctx := context.Background()

	// Prepare queues which should start the listener
	err = b.PrepareWorkflowQueues(ctx, []workflow.Queue{workflow.QueueDefault})
	require.NoError(t, err)

	// Create a workflow instance
	instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	event := history.NewPendingEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
		Metadata: &workflow.Metadata{},
		Queue:    workflow.QueueDefault,
	})

	err = b.CreateWorkflowInstance(ctx, instance, event)
	require.NoError(t, err)

	// GetWorkflowTask should return immediately with notifications
	start := time.Now()
	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	task, err := b.GetWorkflowTask(taskCtx, []workflow.Queue{workflow.QueueDefault})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, task, "should receive task immediately via notification")
	assert.Equal(t, instance.InstanceID, task.WorkflowInstance.InstanceID)

	// Should complete in under 1 second (much faster than polling would take)
	assert.Less(t, elapsed, 1*time.Second, "task should be retrieved quickly via notification")
}

func TestNotifications_ActivityTask(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Create test database
	adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
	require.NoError(t, err)
	defer adminDB.Close()

	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)

	defer func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
	}()

	// Create backend with notifications enabled
	b := NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName,
		WithNotifications(true),
		WithBackendOptions(backend.WithStickyTimeout(0)))
	defer b.Close()

	ctx := context.Background()

	// Prepare queues
	err = b.PrepareActivityQueues(ctx, []workflow.Queue{workflow.QueueDefault})
	require.NoError(t, err)

	// Create a workflow instance first
	instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	event := history.NewPendingEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
		Metadata: &workflow.Metadata{},
		Queue:    workflow.QueueDefault,
	})
	err = b.CreateWorkflowInstance(ctx, instance, event)
	require.NoError(t, err)

	// Schedule an activity
	activityEvent := history.NewPendingEvent(time.Now(), history.EventType_ActivityScheduled, &history.ActivityScheduledAttributes{
		Name: "test-activity",
	})

	tx, err := b.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Insert attributes first (required for activity queries)
	err = insertHistoryEvents(ctx, tx, instance, []*history.Event{activityEvent})
	require.NoError(t, err)

	err = scheduleActivity(ctx, tx, workflow.QueueDefault, instance, activityEvent)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// GetActivityTask should return immediately with notifications
	start := time.Now()
	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	task, err := b.GetActivityTask(taskCtx, []workflow.Queue{workflow.QueueDefault})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, task, "should receive activity task immediately via notification")
	assert.Equal(t, instance.InstanceID, task.WorkflowInstance.InstanceID)

	// Should complete in under 1 second
	assert.Less(t, elapsed, 1*time.Second, "activity task should be retrieved quickly via notification")
}

func TestNotifications_DisabledByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Create test database
	adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
	require.NoError(t, err)
	defer adminDB.Close()

	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)

	defer func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
	}()

	// Create backend WITHOUT notifications enabled
	b := NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName,
		WithBackendOptions(backend.WithStickyTimeout(0)))
	defer b.Close()

	// Listener should be nil when notifications are disabled
	assert.Nil(t, b.listener, "listener should be nil when notifications are disabled")
}

func TestNotifications_ConcurrentWorkflowTasks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Create test database
	adminDB, err := sql.Open("pgx", fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=postgres sslmode=disable", testUser, testPassword))
	require.NoError(t, err)
	defer adminDB.Close()

	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)

	defer func() {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
	}()

	// Create backend with notifications enabled
	b := NewPostgresBackend("localhost", 5432, testUser, testPassword, dbName,
		WithNotifications(true),
		WithBackendOptions(backend.WithStickyTimeout(0)))
	defer b.Close()

	ctx := context.Background()
	err = b.PrepareWorkflowQueues(ctx, []workflow.Queue{workflow.QueueDefault})
	require.NoError(t, err)

	// Start multiple goroutines waiting for tasks
	const numPollers = 3
	taskReceived := make(chan *backend.WorkflowTask, numPollers)

	for i := 0; i < numPollers; i++ {
		go func() {
			taskCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			task, _ := b.GetWorkflowTask(taskCtx, []workflow.Queue{workflow.QueueDefault})
			if task != nil {
				taskReceived <- task
			}
		}()
	}

	// Give pollers time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Create a workflow instance - should notify all waiting pollers
	instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	event := history.NewPendingEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{
		Metadata: &workflow.Metadata{},
		Queue:    workflow.QueueDefault,
	})

	err = b.CreateWorkflowInstance(ctx, instance, event)
	require.NoError(t, err)

	// One poller should get the task quickly
	select {
	case task := <-taskReceived:
		assert.Equal(t, instance.InstanceID, task.WorkflowInstance.InstanceID)
	case <-time.After(2 * time.Second):
		t.Fatal("no poller received the task within timeout")
	}
}
