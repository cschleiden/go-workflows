package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type instanceState struct {
	InstanceID  string                `json:"instance_id,omitempty"`
	ExecutionID string                `json:"execution_id,omitempty"`
	State       backend.WorkflowState `json:"state,omitempty"`
	CreatedAt   time.Time             `json:"created_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
}

func storeInstance(ctx context.Context, rdb redis.UniversalClient, instance core.WorkflowInstance, state *instanceState) error {
	key := instanceKey(instance)

	b, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "could not marshal instance state")
	}

	// TODO: Check individual error here? With pipelining this will only be available once that's set
	cmd := rdb.SetNX(ctx, key, string(b), 0)
	if err := cmd.Err(); err != nil {
		return errors.Wrap(err, "could not store instance")
	}

	if !cmd.Val() {
		return errors.New("workflow instance already exists")
	}

	return nil
}

func readInstance(ctx context.Context, p redis.Pipeliner, instance core.WorkflowInstance) (*instanceState, error) {
	key := instanceKey(instance)
	cmd := p.Get(ctx, key)

	if err := cmd.Err(); err != nil {
		return nil, errors.Wrap(err, "could not read instance")
	}

	var state instanceState
	if err := json.Unmarshal([]byte(cmd.String()), &state); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal instance state")
	}

	return &state, nil
}
