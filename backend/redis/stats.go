package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
)

func (rb *redisBackend) GetStats(ctx context.Context) (*backend.Stats, error) {
	var err error

	s := &backend.Stats{}

	// get workflow instances
	activeInstances, err := rb.rdb.SCard(ctx, rb.keys.instancesActive()).Result()
	if err != nil {
		return nil, fmt.Errorf("getting active instances: %w", err)
	}

	s.ActiveWorkflowInstances = activeInstances

	// get pending workflow tasks
	pendingWorkflows, err := rb.workflowQueue.Size(ctx, rb.rdb)
	if err != nil {
		return nil, fmt.Errorf("getting active workflows: %w", err)
	}

	s.PendingWorkflowTasks = pendingWorkflows

	// get pending activities
	pendingActivities, err := rb.activityQueue.Size(ctx, rb.rdb)
	if err != nil {
		return nil, fmt.Errorf("getting active activities: %w", err)
	}

	s.PendingActivityTasks = pendingActivities

	return s, nil
}
