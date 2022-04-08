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
		return errors.Wrap(err, "could not create event stream")
	}

	// Queue workflow instance task
	if _, err := rb.workflowQueue.Enqueue(ctx, event.WorkflowInstance.InstanceID, &workflowTaskData{
		LastPendingEventMessageID: msgID,
	}); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return errors.Wrap(err, "could not queue workflow task")
		}
	}

	rb.options.Logger.Debug("Created new workflow instance")

	return nil
}

func (rb *redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance) ([]history.Event, error) {
	msgs, err := rb.rdb.XRange(ctx, historyKey(instance.InstanceID), "-", "+").Result()
	if err != nil {
		return nil, err
	}

	var events []history.Event
	for _, msg := range msgs {
		var event history.Event
		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, errors.Wrap(err, "could not unmarshal event")
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
			return errors.Wrap(err, "could not add cancellation event to workflow instance")
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
	Instance    *core.WorkflowInstance `json:"instance,omitempty"`
	State       backend.WorkflowState  `json:"state,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

func createInstance(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance, ignoreDuplicate bool) error {
	key := instanceKey(instance.InstanceID)

	b, err := json.Marshal(&instanceState{
		Instance:  instance,
		State:     backend.WorkflowStateActive,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "could not marshal instance state")
	}

	ok, err := rdb.SetNX(ctx, key, string(b), 0).Result()
	if err != nil {
		return errors.Wrap(err, "could not store instance")
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
			return errors.Wrap(err, "could not track sub-workflow")
		}
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

	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, backend.ErrInstanceNotFound
		}

		return nil, errors.Wrap(err, "could not read instance")
	}

	var state instanceState
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal instance state")
	}

	if state.Instance.SubWorkflow() && state.State == backend.WorkflowStateFinished {
		instanceStr, err := json.Marshal(state.Instance)
		if err != nil {
			return nil, err
		}

		if err := rdb.LRem(ctx, subInstanceKey(state.Instance.ParentInstanceID), 1, instanceStr).Err(); err != nil {
			return nil, errors.Wrap(err, "could not remove sub-workflow from parent list")
		}
	}

	return &state, nil
}

func subWorkflowInstances(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance) ([]*core.WorkflowInstance, error) {
	key := subInstanceKey(instance.InstanceID)
	res, err := rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, errors.Wrap(err, "could not read sub-workflow instances")
	}

	var instances []*core.WorkflowInstance

	for _, instanceStr := range res {
		var instance core.WorkflowInstance
		if err := json.Unmarshal([]byte(instanceStr), &instance); err != nil {
			return nil, errors.Wrap(err, "could not unmarshal sub-workflow instance")
		}

		instances = append(instances, &instance)
	}

	return instances, nil
}
