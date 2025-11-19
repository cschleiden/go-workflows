package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/valkey-io/valkey-go"
)

func (vb *valkeyBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	a := event.Attributes.(*history.ExecutionStartedAttributes)

	instanceState, err := json.Marshal(&instanceState{
		Queue:     string(a.Queue),
		Instance:  instance,
		State:     core.WorkflowInstanceStateActive,
		Metadata:  a.Metadata,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("marshaling instance state: %w", err)
	}

	activeInstance, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("marshaling instance: %w", err)
	}

	eventData, payloadData, err := marshalEvent(event)
	if err != nil {
		return err
	}

	keyInfo := vb.workflowQueue.Keys(a.Queue)

	// Execute Lua script for atomic creation
	err = createWorkflowInstanceScript.Exec(ctx, vb.client, []string{
		vb.keys.instanceKey(instance),
		vb.keys.activeInstanceExecutionKey(instance.InstanceID),
		vb.keys.pendingEventsKey(instance),
		vb.keys.payloadKey(instance),
		vb.keys.instancesActive(),
		vb.keys.instancesByCreation(),
		keyInfo.SetKey,
		keyInfo.StreamKey,
		vb.workflowQueue.queueSetKey,
	}, []string{
		instanceSegment(instance),
		string(instanceState),
		string(activeInstance),
		event.ID,
		eventData,
		payloadData,
		fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	}).Error()

	if err != nil {
		if err.Error() == "ERR InstanceAlreadyExists" {
			return backend.ErrInstanceAlreadyExists
		}
		return fmt.Errorf("creating workflow instance: %w", err)
	}

	return nil
}

func (vb *valkeyBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error) {
	start := "-"
	if lastSequenceID != nil {
		start = strconv.FormatInt(*lastSequenceID, 10)
	}

	msgs, err := vb.client.Do(ctx, vb.client.B().Xrange().Key(vb.keys.historyKey(instance)).Start(start).End("+").Build()).AsXRange()
	if err != nil {
		return nil, err
	}

	payloadKeys := make([]string, 0, len(msgs))
	events := make([]*history.Event, 0, len(msgs))
	for _, msg := range msgs {
		eventStr, ok := msg.FieldValues["event"]
		if !ok || eventStr == "" {
			continue
		}

		var event *history.Event
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		payloadKeys = append(payloadKeys, event.ID)
		events = append(events, event)
	}

	if len(payloadKeys) > 0 {
		cmd := vb.client.B().Hmget().Key(vb.keys.payloadKey(instance)).Field(payloadKeys...)
		res, err := vb.client.Do(ctx, cmd.Build()).AsStrSlice()
		if err != nil {
			return nil, fmt.Errorf("reading payloads: %w", err)
		}

		for i, event := range events {
			event.Attributes, err = history.DeserializeAttributes(event.Type, []byte(res[i]))
			if err != nil {
				return nil, fmt.Errorf("deserializing attributes for event %v: %w", event.Type, err)
			}
		}
	}

	return events, nil
}

func (vb *valkeyBackend) GetWorkflowInstanceState(ctx context.Context, instance *core.WorkflowInstance) (core.WorkflowInstanceState, error) {
	instanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKey(instance))
	if err != nil {
		return core.WorkflowInstanceStateActive, err
	}

	return instanceState.State, nil
}

func (vb *valkeyBackend) CancelWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance, event *history.Event) error {
	// Read the instance to check if it exists
	instanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKey(instance))
	if err != nil {
		return err
	}

	// Prepare event data
	eventData, payloadData, err := marshalEvent(event)
	if err != nil {
		return err
	}

	keyInfo := vb.workflowQueue.Keys(workflow.Queue(instanceState.Queue))

	// Cancel instance
	err = cancelWorkflowInstanceScript.Exec(ctx, vb.client, []string{
		vb.keys.payloadKey(instance),
		vb.keys.pendingEventsKey(instance),
		keyInfo.SetKey,
		keyInfo.StreamKey,
	}, []string{
		event.ID,
		eventData,
		payloadData,
		instanceSegment(instance),
	}).Error()

	if err != nil {
		return fmt.Errorf("canceling workflow instance: %w", err)
	}

	return nil
}

func (vb *valkeyBackend) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	i, err := readInstance(ctx, vb.client, vb.keys.instanceKey(instance))
	if err != nil {
		return err
	}

	if i.State != core.WorkflowInstanceStateFinished && i.State != core.WorkflowInstanceStateContinuedAsNew {
		return backend.ErrInstanceNotFinished
	}

	return vb.deleteInstance(ctx, instance)
}

func (vb *valkeyBackend) RemoveWorkflowInstances(_ context.Context, _ ...backend.RemovalOption) error {
	return backend.ErrNotSupported{
		Message: "not supported, use auto-expiration",
	}
}

type instanceState struct {
	Queue string `json:"queue"`

	Instance *core.WorkflowInstance     `json:"instance,omitempty"`
	State    core.WorkflowInstanceState `json:"state,omitempty"`

	Metadata *metadata.WorkflowMetadata `json:"metadata,omitempty"`

	CreatedAt   time.Time  `json:"created_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	LastSequenceID int64 `json:"last_sequence_id,omitempty"`
}

func readInstance(ctx context.Context, client valkey.Client, instanceKey string) (*instanceState, error) {
	val, err := client.Do(ctx, client.B().Get().Key(instanceKey).Build()).ToString()
	if err != nil {
		return nil, fmt.Errorf("reading instance: %w", err)
	}

	if val == "" {
		return nil, backend.ErrInstanceNotFound
	}

	var state instanceState
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, fmt.Errorf("unmarshaling instance state: %w", err)
	}

	return &state, nil
}

func (vb *valkeyBackend) readActiveInstanceExecution(ctx context.Context, instanceID string) (*core.WorkflowInstance, error) {
	val, err := vb.client.Do(ctx, vb.client.B().Get().Key(vb.keys.activeInstanceExecutionKey(instanceID)).Build()).ToString()
	if err != nil {
		return nil, err
	}

	if val == "" {
		return nil, nil
	}

	var instance *core.WorkflowInstance
	if err := json.Unmarshal([]byte(val), &instance); err != nil {
		return nil, fmt.Errorf("unmarshaling instance: %w", err)
	}

	return instance, nil
}
