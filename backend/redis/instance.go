package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func (rb *redisBackend) CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error {
	// Store instance with its state
	if err := createInstance(ctx, rb.rdb, event.WorkflowInstance, &instanceState{
		InstanceID:  event.WorkflowInstance.GetInstanceID(),
		ExecutionID: event.WorkflowInstance.GetExecutionID(),
		State:       backend.WorkflowStateActive,
		CreatedAt:   time.Now(),
	}); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	// Create event stream
	eventData, err := json.Marshal(event.HistoryEvent)
	if err != nil {
		return err
	}

	cmd := rb.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: pendingEventsKey(event.WorkflowInstance.GetInstanceID()),
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	})
	_, err = cmd.Result()
	if err != nil {
		return errors.Wrap(err, "could not create event stream")
	}

	// Add instance to pending instances set
	if _, err := rb.workflowQueue.Enqueue(ctx, event.WorkflowInstance.GetInstanceID(), nil); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return errors.Wrap(err, "could not queue workflow task")
		}
	}

	return nil
}

func (rb *redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance core.WorkflowInstance) ([]history.Event, error) {
	panic("unimplemented")
}

func (rb *redisBackend) GetWorkflowInstanceState(ctx context.Context, instance core.WorkflowInstance) (backend.WorkflowState, error) {
	instanceState, err := readInstance(ctx, rb.rdb, instance.GetInstanceID())
	if err != nil {
		return backend.WorkflowStateActive, err
	}

	return instanceState.State, nil
}

func (rb *redisBackend) CancelWorkflowInstance(ctx context.Context, instance core.WorkflowInstance) error {
	panic("unimplemented")
}

type instanceState struct {
	InstanceID  string                `json:"instance_id,omitempty"`
	ExecutionID string                `json:"execution_id,omitempty"`
	State       backend.WorkflowState `json:"state,omitempty"`
	CreatedAt   time.Time             `json:"created_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
}

func createInstance(ctx context.Context, rdb redis.UniversalClient, instance core.WorkflowInstance, state *instanceState) error {
	key := instanceKey(instance.GetInstanceID())

	b, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "could not marshal instance state")
	}

	ok, err := rdb.SetNX(ctx, key, string(b), 0).Result()
	if err != nil {
		return errors.Wrap(err, "could not store instance")
	}

	if !ok {
		return errors.New("workflow instance already exists")
	}

	return nil
}

func updateInstance(ctx context.Context, rdb redis.UniversalClient, instanceID string, state *instanceState) error {
	key := instanceKey(instanceID)

	b, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "could not marshal instance state")
	}

	cmd := rdb.Set(ctx, key, string(b), 0)
	if err := cmd.Err(); err != nil {
		return errors.Wrap(err, "could not update instance")
	}

	return nil
}

func readInstance(ctx context.Context, rdb redis.UniversalClient, instanceID string) (*instanceState, error) {
	key := instanceKey(instanceID)
	cmd := rdb.Get(ctx, key)

	if err := cmd.Err(); err != nil {
		return nil, errors.Wrap(err, "could not read instance")
	}

	var state instanceState
	if err := json.Unmarshal([]byte(cmd.Val()), &state); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal instance state")
	}

	return &state, nil
}
