package valkey

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/valkey-io/valkey-glide/go/v2/constants"
	"github.com/valkey-io/valkey-glide/go/v2/options"
)

var _ diag.Backend = (*valkeyBackend)(nil)

func (vb *valkeyBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID, afterExecutionID string, count int) ([]*diag.WorkflowInstanceRef, error) {
	start := options.NewInclusiveScoreBoundary(0)
	end := options.NewInfiniteScoreBoundary(constants.PositiveInfinity)

	zrangeInput := &options.RangeByScore{
		Start:   start,
		End:     end,
		Reverse: true,
		Limit: &options.Limit{
			Offset: 0,
			Count:  int64(count),
		},
	}

	if afterInstanceID != "" {
		afterSegmentID := instanceSegment(core.NewWorkflowInstance(afterInstanceID, afterExecutionID))
		scores, err := vb.client.ZMScore(ctx, vb.keys.instancesByCreation(), []string{afterSegmentID})
		if err != nil {
			return nil, fmt.Errorf("getting instance score for %v: %w", afterSegmentID, err)
		}

		if len(scores) == 0 {
			vb.Options().Logger.Error("could not find instance %v",
				log.NamespaceKey+".valkey.afterInstanceID", afterInstanceID,
				log.NamespaceKey+".valkey.afterExecutionID", afterExecutionID,
			)
			return nil, nil
		}

		end := options.NewScoreBoundary(scores[0].Value(), false)
		zrangeInput.End = end
	}

	instanceSegments, err := vb.client.ZRange(ctx, vb.keys.instancesByCreation(), zrangeInput)
	if err != nil {
		return nil, fmt.Errorf("getting instances: %w", err)
	}

	if len(instanceSegments) == 0 {
		return nil, nil
	}

	instanceKeys := make([]string, 0)
	for _, r := range instanceSegments {
		instanceKeys = append(instanceKeys, vb.keys.instanceKeyFromSegment(r))
	}

	instances, err := vb.client.MGet(ctx, instanceKeys)
	if err != nil {
		return nil, fmt.Errorf("getting instances: %w", err)
	}

	instanceRefs := make([]*diag.WorkflowInstanceRef, 0, len(instances))
	for _, instance := range instances {
		if instance.IsNil() {
			continue
		}

		var state instanceState
		if err := json.Unmarshal([]byte(instance.Value()), &state); err != nil {
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

func (vb *valkeyBackend) GetWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) (*diag.WorkflowInstanceRef, error) {
	instanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKey(instance))
	if err != nil {
		return nil, err
	}

	return mapWorkflowInstance(instanceState), nil
}

func (vb *valkeyBackend) GetWorkflowTree(ctx context.Context, instance *core.WorkflowInstance) (*diag.WorkflowInstanceTree, error) {
	itb := diag.NewInstanceTreeBuilder(vb)
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
