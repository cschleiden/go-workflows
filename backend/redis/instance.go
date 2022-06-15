package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func (rb *redisBackend) CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error {
	state, err := readInstance(ctx, rb.rdb, event.WorkflowInstance.InstanceID)
	if err != nil && err != backend.ErrInstanceNotFound {
		return err
	}

	if state != nil {
		return backend.ErrInstanceAlreadyExists
	}

	p := rb.rdb.TxPipeline()

	if err := createInstanceP(ctx, p, event.WorkflowInstance, false); err != nil {
		return err
	}

	// Create event stream
	eventData, err := json.Marshal(event.HistoryEvent)
	if err != nil {
		return err
	}

	p.XAdd(ctx, &redis.XAddArgs{
		Stream: pendingEventsKey(event.WorkflowInstance.InstanceID),
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	})

	// Queue workflow instance task
	if err := rb.workflowQueue.Enqueue(ctx, p, event.WorkflowInstance.InstanceID, nil); err != nil {
		if err != errTaskAlreadyInQueue {
			return fmt.Errorf("queueing workflow task: %w", err)
		}
	}

	if _, err := p.Exec(ctx); err != nil {
		return fmt.Errorf("creating workflow instance: %w", err)
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
	// Read the instance to check if it exists
	_, err := readInstance(ctx, rb.rdb, instance.InstanceID)
	if err != nil {
		return err
	}

	// Cancel instance
	if cmds, err := rb.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		return rb.addWorkflowInstanceEventP(ctx, p, instance, event)
	}); err != nil {
		fmt.Println(cmds)
		return fmt.Errorf("adding cancellation event to workflow instance: %w", err)
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

func createInstanceP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, ignoreDuplicate bool) error {
	key := instanceKey(instance.InstanceID)

	createdAt := time.Now()

	b, err := json.Marshal(&instanceState{
		Instance:  instance,
		State:     backend.WorkflowStateActive,
		CreatedAt: createdAt,
	})
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	p.SetNX(ctx, key, string(b), 0)

	p.ZAdd(ctx, instancesByCreation(), &redis.Z{
		Member: instance.InstanceID,
		Score:  float64(createdAt.UnixMilli()),
	})

	return nil
}

func updateInstanceP(ctx context.Context, p redis.Pipeliner, instanceID string, state *instanceState) error {
	key := instanceKey(instanceID)

	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	p.Set(ctx, key, string(b), 0)

	// CreatedAt does not change, so skip updating the instancesByCreation() ZSET

	return nil
}

func readInstance(ctx context.Context, rdb redis.UniversalClient, instanceID string) (*instanceState, error) {
	p := rdb.Pipeline()

	cmd := readInstanceP(ctx, p, instanceID)

	// Error is checked when checking the cmd
	_, _ = p.Exec(ctx)

	return readInstancePipelineCmd(cmd)
}

func readInstanceP(ctx context.Context, p redis.Pipeliner, instanceID string) *redis.StringCmd {
	key := instanceKey(instanceID)

	return p.Get(ctx, key)
}

func readInstancePipelineCmd(cmd *redis.StringCmd) (*instanceState, error) {
	val, err := cmd.Result()
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

	return &state, nil
}
