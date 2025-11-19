package valkey

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/log"
)

var _ diag.Backend = (*valkeyBackend)(nil)

func (vb *valkeyBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID, afterExecutionID string, count int) ([]*diag.WorkflowInstanceRef, error) {
	zrangeCmd := vb.client.B().Zrange().Key(vb.keys.instancesByCreation()).Min("0").Max("-1").Rev().Limit(0, int64(count))

	if afterInstanceID != "" {
		afterSegmentID := instanceSegment(core.NewWorkflowInstance(afterInstanceID, afterExecutionID))
		scores, err := vb.client.Do(ctx, vb.client.B().Zscore().Key(vb.keys.instancesByCreation()).Member(afterSegmentID).Build()).AsFloat64()
		if err != nil {
			return nil, fmt.Errorf("getting instance score for %v: %w", afterSegmentID, err)
		}

		if scores == 0 {
			vb.Options().Logger.Error("could not find instance %v",
				log.NamespaceKey+".valkey.afterInstanceID", afterInstanceID,
				log.NamespaceKey+".valkey.afterExecutionID", afterExecutionID,
			)
			return nil, nil
		}

		zrangeCmd = vb.client.B().Zrange().Key(vb.keys.instancesByCreation()).Min("-inf").Max(fmt.Sprintf("(%f", scores)).Rev().Limit(0, int64(count))
	}

	instanceSegments, err := vb.client.Do(ctx, zrangeCmd.Build()).AsStrSlice()
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

	cmd := vb.client.B().Mget().Key(instanceKeys...)
	instances, err := vb.client.Do(ctx, cmd.Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("getting instances: %w", err)
	}

	instanceRefs := make([]*diag.WorkflowInstanceRef, 0, len(instances))
	for _, instance := range instances {
		if instance == "" {
			continue
		}

		var state instanceState
		if err := json.Unmarshal([]byte(instance), &state); err != nil {
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
