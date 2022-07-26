package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/internal/core"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ diag.Backend = (*mongoBackend)(nil)

// GetWorkflowInstances returns up to 'count' workflow instances later than
// the creation date of 'afterInstanceID'.
func (b *mongoBackend) GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*diag.WorkflowInstanceRef, error) {

	filter := bson.M{}

	// build filter using afterInstanceID's created_at dtime
	if afterInstanceID != "" {
		var inst instance
		if err := b.db.Collection("instances").FindOne(ctx, bson.M{"instance_id": afterInstanceID}).Decode(&inst); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, errors.New("was expecting to find an event")
			}
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}
		filter = bson.M{"$or": bson.A{
			bson.M{"created_at": bson.M{"$lt": inst.CreatedAt}},
			bson.M{"$and": bson.A{
				bson.M{"created_at": inst.CreatedAt},
				bson.M{"instance_id": bson.M{"$lt": inst.InstanceID}},
			}},
		}}
	}

	// find matching instances
	lim := int64(count)
	opts := options.FindOptions{
		Limit: &lim,
		Sort: bson.A{
			bson.M{"created_at": -1},
			bson.M{"sequence_id": -1},
		},
	}
	cursor, err := b.db.Collection("instances").Find(ctx, filter, &opts)
	if err != nil {
		return nil, err
	}
	var insts []instance
	if err := cursor.All(ctx, &insts); err != nil {
		return nil, err
	}

	// unpack results
	instances := make([]*diag.WorkflowInstanceRef, len(insts))
	for i := 0; i < len(insts); i++ {
		var state backend.WorkflowState
		if insts[i].CompletedAt != nil {
			state = backend.WorkflowStateFinished
		}
		instances[i] = &diag.WorkflowInstanceRef{
			Instance:    core.NewWorkflowInstance(insts[i].InstanceID, insts[i].ExecutionID),
			CreatedAt:   insts[i].CreatedAt,
			CompletedAt: insts[i].CompletedAt,
			State:       state,
		}
	}

	return instances, nil
}

// GetWorkflowInstance returns the workflow instance with the given id.
func (b *mongoBackend) GetWorkflowInstance(ctx context.Context, instanceID string) (*diag.WorkflowInstanceRef, error) {
	var inst instance
	if err := b.db.Collection("instances").FindOne(ctx, bson.M{"instance_id": instanceID}).Decode(&inst); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, backend.ErrInstanceNotFound
		}
		return nil, err
	}

	var state backend.WorkflowState
	if inst.CompletedAt != nil {
		state = backend.WorkflowStateFinished
	}

	return &diag.WorkflowInstanceRef{
		Instance:    core.NewWorkflowInstance(instanceID, inst.ExecutionID),
		CreatedAt:   inst.CreatedAt,
		CompletedAt: inst.CompletedAt,
		State:       state,
	}, nil
}
