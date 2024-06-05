package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/log"
	redis "github.com/redis/go-redis/v9"
)

var _ diag.Backend = (*redisBackend)(nil)

func (rb *redisBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID, afterExecutionID string, count int) ([]*diag.WorkflowInstanceRef, error) {
	max := "+inf"

	if afterInstanceID != "" {
		afterSegmentID := instanceSegment(core.NewWorkflowInstance(afterInstanceID, afterExecutionID))
		scores, err := rb.rdb.ZMScore(ctx, rb.keys.instancesByCreation(), afterSegmentID).Result()
		if err != nil {
			return nil, fmt.Errorf("getting instance score for %v: %w", afterSegmentID, err)
		}

		if len(scores) == 0 {
			rb.Options().Logger.Error("could not find instance %v",
				log.NamespaceKey+".redis.afterInstanceID", afterInstanceID,
				log.NamespaceKey+".redis.afterExecutionID", afterExecutionID,
			)
			return nil, nil
		}

		max = fmt.Sprintf("(%v", int64(scores[0]))
	}

	result, err := rb.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     rb.keys.instancesByCreation(),
		Stop:    max,
		Start:   "-inf",
		ByScore: true,
		Rev:     true,
		Count:   int64(count),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("getting instances after %v: %w", max, err)
	}

	instanceKeys := make([]string, 0)
	for _, r := range result {
		instanceSegment := r
		instanceKeys = append(instanceKeys, rb.keys.instanceKeyFromSegment(instanceSegment))
	}

	if len(instanceKeys) == 0 {
		return nil, nil
	}

	instances, err := rb.rdb.MGet(ctx, instanceKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("getting instances: %w", err)
	}

	var instanceRefs []*diag.WorkflowInstanceRef
	for _, instance := range instances {
		instStr, ok := instance.(string)
		if !ok {
			continue
		}

		var state instanceState
		if err := json.Unmarshal([]byte(instStr), &state); err != nil {
			return nil, fmt.Errorf("unmarshaling instance state: %w", err)
		}

		instanceRefs = append(instanceRefs, &diag.WorkflowInstanceRef{
			Instance:    state.Instance,
			CreatedAt:   state.CreatedAt,
			CompletedAt: state.CompletedAt,
			State:       state.State,
			Queue:       state.Queue,
		})
	}

	return instanceRefs, nil
}

func (rb *redisBackend) GetWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) (*diag.WorkflowInstanceRef, error) {
	instanceState, err := readInstance(ctx, rb.rdb, rb.keys.instanceKey(instance))
	if err != nil {
		return nil, err
	}

	return mapWorkflowInstance(instanceState), nil
}

func (rb *redisBackend) GetWorkflowTree(ctx context.Context, instance *core.WorkflowInstance) (*diag.WorkflowInstanceTree, error) {
	itb := diag.NewInstanceTreeBuilder(rb)
	return itb.BuildWorkflowInstanceTree(ctx, instance)
}

func mapWorkflowInstance(instance *instanceState) *diag.WorkflowInstanceRef {
	return &diag.WorkflowInstanceRef{
		Instance:    instance.Instance,
		CreatedAt:   instance.CreatedAt,
		CompletedAt: instance.CompletedAt,
		State:       instance.State,
		Queue:       instance.Queue,
	}
}
