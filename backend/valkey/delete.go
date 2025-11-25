package valkey

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/core"
)

// deleteInstance deletes an instance from Valkey. It does not attempt to remove any future events or pending
// workflow tasks. It's assumed that the instance is in the finished state.
//
// Note: might want to revisit this in the future if we want to support removing hung instances.
func (vb *valkeyBackend) deleteInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	err := deleteInstanceScript.Exec(ctx, vb.client, []string{
		vb.keys.instanceKey(instance),
		vb.keys.pendingEventsKey(instance),
		vb.keys.historyKey(instance),
		vb.keys.payloadKey(instance),
		vb.keys.activeInstanceExecutionKey(instance.InstanceID),
		vb.keys.instancesByCreation(),
	}, []string{
		instanceSegment(instance),
	}).Error()

	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	return nil
}
