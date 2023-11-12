package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/redis/go-redis/v9"
)

func (rb *redisBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	activeInstance, err := readActiveInstanceExecution(ctx, rb.rdb, instance.InstanceID)
	if err != nil && err != backend.ErrInstanceNotFound {
		return err
	}

	if activeInstance != nil {
		return backend.ErrInstanceAlreadyExists
	}

	p := rb.rdb.TxPipeline()

	if err := createInstanceP(ctx, p, instance, event.Attributes.(*history.ExecutionStartedAttributes).Metadata, false); err != nil {
		return err
	}

	// Create event stream with initial event
	if err := addEventPayloadsP(ctx, p, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("adding event payloads: %w", err)
	}

	if err := addEventToStreamP(ctx, p, pendingEventsKey(instance), event); err != nil {
		return fmt.Errorf("adding event to stream: %w", err)
	}

	// Queue workflow instance task
	if err := rb.workflowQueue.Enqueue(ctx, p, instanceSegment(instance), nil); err != nil {
		return fmt.Errorf("queueing workflow task: %w", err)
	}

	if _, err := p.Exec(ctx); err != nil {
		return fmt.Errorf("creating workflow instance: %w", err)
	}

	rb.options.Logger.Debug("Created new workflow instance")

	return nil
}

func (rb *redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error) {
	start := "-"

	if lastSequenceID != nil {
		start = "(" + historyID(*lastSequenceID)
	}

	msgs, err := rb.rdb.XRange(ctx, historyKey(instance), start, "+").Result()
	if err != nil {
		return nil, err
	}

	payloadKeys := make([]string, 0, len(msgs))
	var events []*history.Event
	for _, msg := range msgs {
		var event *history.Event
		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		payloadKeys = append(payloadKeys, event.ID)
		events = append(events, event)
	}

	res, err := rb.rdb.HMGet(ctx, payloadKey(instance), payloadKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("reading payloads: %w", err)
	}

	for i, event := range events {
		event.Attributes, err = history.DeserializeAttributes(event.Type, []byte(res[i].(string)))
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes for event %v: %w", event.Type, err)
		}
	}

	return events, nil
}

func (rb *redisBackend) GetWorkflowInstanceState(ctx context.Context, instance *core.WorkflowInstance) (core.WorkflowInstanceState, error) {
	instanceState, err := readInstance(ctx, rb.rdb, instanceKey(instance))
	if err != nil {
		return core.WorkflowInstanceStateActive, err
	}

	return instanceState.State, nil
}

func (rb *redisBackend) CancelWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance, event *history.Event) error {
	// Read the instance to check if it exists
	_, err := readInstance(ctx, rb.rdb, instanceKey(instance))
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

func (rb *redisBackend) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	i, err := readInstance(ctx, rb.rdb, instanceKey(instance))
	if err != nil {
		return err
	}

	// Check state
	if i.State != core.WorkflowInstanceStateFinished {
		return backend.ErrInstanceNotFinished
	}

	return deleteInstance(ctx, rb.rdb, instance)
}

type instanceState struct {
	Instance *core.WorkflowInstance     `json:"instance,omitempty"`
	State    core.WorkflowInstanceState `json:"state,omitempty"`

	Metadata *metadata.WorkflowMetadata `json:"metadata,omitempty"`

	CreatedAt   time.Time  `json:"created_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	LastSequenceID int64 `json:"last_sequence_id,omitempty"`
}

func createInstanceP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, metadata *metadata.WorkflowMetadata, ignoreDuplicate bool) error {
	key := instanceKey(instance)

	createdAt := time.Now()

	b, err := json.Marshal(&instanceState{
		Instance:  instance,
		State:     core.WorkflowInstanceStateActive,
		Metadata:  metadata,
		CreatedAt: createdAt,
	})
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	p.SetNX(ctx, key, string(b), 0)

	// The newly created instance is going to be the active execution
	setActiveInstanceExecutionP(ctx, p, instance)

	p.ZAdd(ctx, instancesByCreation(), redis.Z{
		Member: instanceSegment(instance),
		Score:  float64(createdAt.UnixMilli()),
	})

	p.SAdd(ctx, instancesActive(), instanceSegment(instance))

	return nil
}

func updateInstanceP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, state *instanceState) error {
	key := instanceKey(instance)

	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	p.Set(ctx, key, string(b), 0)

	if state.State != core.WorkflowInstanceStateActive {
		p.SRem(ctx, instancesActive(), instanceSegment(instance))
	}

	// CreatedAt does not change, so skip updating the instancesByCreation() ZSET

	return nil
}

func readInstance(ctx context.Context, rdb redis.UniversalClient, instanceKey string) (*instanceState, error) {
	p := rdb.Pipeline()

	cmd := readInstanceP(ctx, p, instanceKey)

	// Error is checked when checking the cmd
	_, _ = p.Exec(ctx)

	return readInstancePipelineCmd(cmd)
}

func readInstanceP(ctx context.Context, p redis.Pipeliner, instanceKey string) *redis.StringCmd {
	return p.Get(ctx, instanceKey)
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

func readActiveInstanceExecution(ctx context.Context, rdb redis.UniversalClient, instanceID string) (*core.WorkflowInstance, error) {
	val, err := rdb.Get(ctx, activeInstanceExecutionKey(instanceID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}

		return nil, err
	}

	var instance *core.WorkflowInstance
	if err := json.Unmarshal([]byte(val), &instance); err != nil {
		return nil, fmt.Errorf("unmarshaling instance: %w", err)
	}

	return instance, nil
}

func setActiveInstanceExecutionP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance) error {
	key := activeInstanceExecutionKey(instance.InstanceID)

	b, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("marshaling instance: %w", err)
	}

	return p.Set(ctx, key, string(b), 0).Err()
}

func removeActiveInstanceExecutionP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance) error {
	key := activeInstanceExecutionKey(instance.InstanceID)

	return p.Del(ctx, key).Err()
}
