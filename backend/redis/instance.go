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
	keyInfo := rb.workflowQueue.Keys()

	instanceState, err := json.Marshal(&instanceState{
		Instance:  instance,
		State:     core.WorkflowInstanceStateActive,
		Metadata:  event.Attributes.(*history.ExecutionStartedAttributes).Metadata,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	activeInstance, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("marshaling instance: %w", err)
	}

	eventData, err := marshalEventWithoutAttributes(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	payloadData, err := json.Marshal(event.Attributes)
	if err != nil {
		return fmt.Errorf("marshaling event payload: %w", err)
	}

	_, err = createWorkflowInstanceCmd.Run(ctx, rb.rdb, []string{
		instanceKey(instance),
		activeInstanceExecutionKey(instance.InstanceID),
		pendingEventsKey(instance),
		payloadKey(instance),
		instancesActive(),
		keyInfo.SetKey,
		keyInfo.StreamKey,
	},
		instanceSegment(instance),
		string(instanceState),
		string(activeInstance),
		event.ID,
		eventData,
		payloadData,
	).Result()

	if err != nil {
		if _, ok := err.(redis.Error); ok {
			if err.Error() == "ERR InstanceAlreadyExists" {
				return backend.ErrInstanceAlreadyExists
			}
		}

		return fmt.Errorf("creating workflow instance: %w", err)
	}

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

func createInstanceP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, metadata *metadata.WorkflowMetadata) error {
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
	if err := setActiveInstanceExecutionP(ctx, p, instance); err != nil {
		return fmt.Errorf("setting active instance execution: %w", err)
	}

	p.ZAdd(ctx, instancesByCreation(), redis.Z{
		Member: instanceSegment(instance),
		Score:  float64(createdAt.UnixMilli()),
	})

	p.SAdd(ctx, instancesActive(), instanceSegment(instance))

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
