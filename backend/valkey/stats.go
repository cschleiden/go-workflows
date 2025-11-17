package valkey

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
)

func (vb *valkeyBackend) GetStats(ctx context.Context) (*backend.Stats, error) {
	s := &backend.Stats{}

	// get workflow instances
	activeInstances, err := vb.client.SCard(ctx, vb.keys.instancesActive())
	if err != nil {
		return nil, fmt.Errorf("getting active instances: %w", err)
	}

	s.ActiveWorkflowInstances = activeInstances

	// get pending workflow tasks
	pendingWorkflows, err := vb.workflowQueue.Size(ctx, vb.client)
	if err != nil {
		return nil, fmt.Errorf("getting active workflows: %w", err)
	}

	s.PendingWorkflowTasks = pendingWorkflows

	// get pending activities
	pendingActivities, err := vb.activityQueue.Size(ctx, vb.client)
	if err != nil {
		return nil, fmt.Errorf("getting active activities: %w", err)
	}

	s.PendingActivityTasks = pendingActivities

	return s, nil
}
