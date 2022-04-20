package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func (rb *redisBackend) CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error {
	if err := createInstance(ctx, rb.rdb, event.WorkflowInstance, false); err != nil {
		return err
	}

	// Create event stream
	eventData, err := json.Marshal(event.HistoryEvent)
	if err != nil {
		return err
	}

	msgID, err := rb.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: pendingEventsKey(event.WorkflowInstance.InstanceID),
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("creating event stream: %w", err)
	}

	// Queue workflow instance task
	if _, err := rb.workflowQueue.Enqueue(ctx, event.WorkflowInstance.InstanceID, &workflowTaskData{
		LastPendingEventMessageID: msgID,
	}); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return fmt.Errorf("queueing workflow task: %w", err)
		}
	}

	rb.options.Logger.Debug("Created new workflow instance")

	return nil
}

func (rb *redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]history.Event, error) {
	msgs, err := rb.rdb.XRange(ctx, historyKey(instance.InstanceID), "-", "+").Result()
	if err != nil {
		return nil, err
	}

	var events []history.Event
	for _, msg := range msgs {
		var event history.Event
		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

func (rb *redisBackend) GetWorkflowInstanceState(ctx context.Context, instance *core.WorkflowInstance) (backend.WorkflowState, error) {
	instanceState, err := readInstance(ctx, rb.rdb, instance.InstanceID)
	if err != nil {
		return backend.WorkflowStateActive, err
	}

	return instanceState.State, nil
}

func (rb *redisBackend) CancelWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance, event *history.Event) error {
	// Recursively, find any sub-workflow instance to cancel
	toCancel := make([]*core.WorkflowInstance, 0)
	toCancel = append(toCancel, instance)
	for len(toCancel) > 0 {
		instance := toCancel[0]
		toCancel = toCancel[1:]

		// Cancel instance
		if err := rb.addWorkflowInstanceEvent(ctx, instance, event); err != nil {
			return fmt.Errorf("adding cancellation event to workflow instance: %w", err)
		}

		// Find sub-workflows
		subInstances, err := subWorkflowInstances(ctx, rb.rdb, instance)
		if err != nil {
			return err
		}

		toCancel = append(toCancel, subInstances...)
	}

	return nil
}

type instanceState struct {
	Instance       *core.WorkflowInstance `json:"instance,omitempty"`
	State          backend.WorkflowState  `json:"state,omitempty"`
	CreatedAt      time.Time              `json:"created_at,omitempty"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	LastSequenceID int64                  `json:"last_sequence_id,omitempty"`
}

func createInstance(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance, ignoreDuplicate bool) error {
	key := instanceKey(instance.InstanceID)

	b, err := json.Marshal(&instanceState{
		Instance:  instance,
		State:     backend.WorkflowStateActive,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	ok, err := rdb.SetNX(ctx, key, string(b), 0).Result()
	if err != nil {
		return fmt.Errorf("storeing instance: %w", err)
	}

	if !ignoreDuplicate && !ok {
		return errors.New("workflow instance already exists")
	}

	if instance.SubWorkflow() {
		instanceStr, err := json.Marshal(instance)
		if err != nil {
			return err
		}

		if err := rdb.RPush(ctx, subInstanceKey(instance.ParentInstanceID), instanceStr).Err(); err != nil {
			return fmt.Errorf("tracking sub-workflow: %w", err)
		}
	}

	return nil
}

func updateInstance(ctx context.Context, rdb redis.UniversalClient, instanceID string, state *instanceState) error {
	key := instanceKey(instanceID)

	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	cmd := rdb.Set(ctx, key, string(b), 0)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("updating instance: %w", err)
	}

	return nil
}

func readInstance(ctx context.Context, rdb redis.UniversalClient, instanceID string) (*instanceState, error) {
	key := instanceKey(instanceID)

	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, backend.ErrInstanceNotFound
		}

		return nil, fmt.Errorf("reading instance: %w", err)
	}

	var state instanceState
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, fmt.Errorf("unmarshaling instance state: %w", err)
	}

	if state.Instance.SubWorkflow() && state.State == backend.WorkflowStateFinished {
		instanceStr, err := json.Marshal(state.Instance)
		if err != nil {
			return nil, err
		}

		if err := rdb.LRem(ctx, subInstanceKey(state.Instance.ParentInstanceID), 1, instanceStr).Err(); err != nil {
			return nil, fmt.Errorf("removing sub-workflow from parent list: %w", err)
		}
	}

	return &state, nil
}

func subWorkflowInstances(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance) ([]*core.WorkflowInstance, error) {
	key := subInstanceKey(instance.InstanceID)
	res, err := rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("reading sub-workflow instances: %w", err)
	}

	var instances []*core.WorkflowInstance

	for _, instanceStr := range res {
		var instance core.WorkflowInstance
		if err := json.Unmarshal([]byte(instanceStr), &instance); err != nil {
			return nil, fmt.Errorf("unmarshaling sub-workflow instance: %w", err)
		}

		instances = append(instances, &instance)
	}

	return instances, nil
}
