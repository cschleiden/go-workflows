package valkey

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/valkey-io/valkey-glide/go/v2/options"
)

func (vb *valkeyBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	// Get current execution of the instance
	instance, err := vb.readActiveInstanceExecution(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("reading active instance execution: %w", err)
	}

	if instance == nil {
		return backend.ErrInstanceNotFound
	}

	instanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKey(instance))
	if err != nil {
		return err
	}

	eventData, payload, err := marshalEvent(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	queue := workflow.Queue(instanceState.Queue)
	queueKeys := vb.workflowQueue.Keys(queue)

	keys := []string{
		vb.keys.payloadKey(instanceState.Instance),
		vb.keys.pendingEventsKey(instanceState.Instance),
		queueKeys.SetKey,
		queueKeys.StreamKey,
	}

	args := []string{
		event.ID,
		eventData,
		payload,
		instanceSegment(instanceState.Instance),
	}

	// Execute the Lua script
	_, err = vb.client.InvokeScriptWithOptions(ctx, signalWorkflowScript, options.ScriptOptions{
		Keys: keys,
		Args: args,
	})
	if err != nil {
		return fmt.Errorf("signaling workflow: %w", err)
	}

	return nil
}
