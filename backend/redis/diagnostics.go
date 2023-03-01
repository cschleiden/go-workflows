package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-workflows/diag"
	redis "github.com/redis/go-redis/v9"
)

var _ diag.Backend = (*redisBackend)(nil)

func (rb *redisBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*diag.WorkflowInstanceRef, error) {
	max := "+inf"

	if afterInstanceID != "" {
		scores, err := rb.rdb.ZMScore(ctx, instancesByCreation(), afterInstanceID).Result()
		if err != nil {
			return nil, fmt.Errorf("getting instance score for %v: %w", afterInstanceID, err)
		}

		if len(scores) == 0 {
			rb.Logger().Error("could not find instance %v", "afterInstanceID", afterInstanceID)
			return nil, nil
		}

		max = fmt.Sprintf("(%v", int64(scores[0]))
	}

	result, err := rb.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     instancesByCreation(),
		Stop:    max,
		Start:   "-inf",
		ByScore: true,
		Rev:     true,
		Count:   int64(count),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("getting instances after %v: %w", max, err)
	}

	instanceIDs := make([]string, 0)
	for _, r := range result {
		instanceID := r
		instanceIDs = append(instanceIDs, instanceKey(instanceID))
	}

	instances, err := rb.rdb.MGet(ctx, instanceIDs...).Result()
	if err != nil {
		return nil, fmt.Errorf("getting instances: %w", err)
	}

	var instanceRefs []*diag.WorkflowInstanceRef
	for _, instance := range instances {
		var state instanceState
		if err := json.Unmarshal([]byte(instance.(string)), &state); err != nil {
			return nil, fmt.Errorf("unmarshaling instance state: %w", err)
		}

		instanceRefs = append(instanceRefs, &diag.WorkflowInstanceRef{
			Instance:    state.Instance,
			CreatedAt:   state.CreatedAt,
			CompletedAt: state.CompletedAt,
			State:       state.State,
		})
	}

	return instanceRefs, nil
}

func (rb *redisBackend) GetWorkflowInstance(ctx context.Context, instanceID string) (*diag.WorkflowInstanceRef, error) {
	instance, err := readInstance(ctx, rb.rdb, instanceID)
	if err != nil {
		return nil, err
	}

	return mapWorkflowInstance(instance), nil
}

func (rb *redisBackend) GetWorkflowTree(ctx context.Context, instanceID string) (*diag.WorkflowInstanceTree, error) {
	itb := diag.NewInstanceTreeBuilder(rb)
	return itb.BuildWorkflowInstanceTree(ctx, instanceID)
}

func mapWorkflowInstance(instance *instanceState) *diag.WorkflowInstanceRef {
	return &diag.WorkflowInstanceRef{
		Instance:    instance.Instance,
		CreatedAt:   instance.CreatedAt,
		CompletedAt: instance.CompletedAt,
		State:       instance.State,
	}
}
