package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/internal/core"
	"github.com/ticctech/go-workflows/internal/history"
	"github.com/ticctech/go-workflows/internal/task"
	"github.com/ticctech/go-workflows/log"
	"github.com/ticctech/go-workflows/workflow"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.opentelemetry.io/otel/trace"
)

type MongoOptions struct {
	backend.Options
	BlockTimeout time.Duration
}

type MongoBackendOption func(*MongoOptions)

func WithBlockTimeout(timeout time.Duration) MongoBackendOption {
	return func(o *MongoOptions) {
		o.BlockTimeout = timeout
	}
}

func WithBackendOptions(opts ...backend.BackendOption) MongoBackendOption {
	return func(o *MongoOptions) {
		for _, opt := range opts {
			opt(&o.Options)
		}
	}
}

type mongoBackend struct {
	db         *mongo.Database
	workerName string
	options    *MongoOptions
}

func (b *mongoBackend) Logger() log.Logger {
	return b.options.Logger
}

func (b *mongoBackend) Tracer() trace.Tracer {
	return b.options.TracerProvider.Tracer(backend.TracerName)
}

func NewMongoBackend(uri, app string, opts ...MongoBackendOption) (*mongoBackend, error) {
	// connect to db
	client, err := mongo.Connect(
		context.Background(),
		options.Client().ApplyURI(uri),
		options.Client().SetAppName(app),
		options.Client().SetConnectTimeout(2*time.Second),
		options.Client().SetReadConcern(readconcern.Local()),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to mongo: %w", err)
	}

	// apply options
	options := &MongoOptions{
		Options:      backend.ApplyOptions(),
		BlockTimeout: time.Second * 2,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &mongoBackend{
		db:         client.Database(app),
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
		options:    options,
	}, nil
}

// CreateWorkflowInstance creates a new workflow instance
func (b *mongoBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event history.Event) error {
	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// create workflow instance
		if err := b.createInstance(sessCtx, instance, event, false); err != nil {
			return nil, err
		}

		// initial history is empty, add new event
		if err := b.insertEvents(sessCtx, "pending_events", instance.InstanceID, []history.Event{event}); err != nil {
			return nil, fmt.Errorf("inserting new event: %w", err)
		}
		return nil, nil
	}

	// execute
	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

// create a workflow instance
func (b *mongoBackend) createInstance(sessCtx mongo.SessionContext, wfi *workflow.Instance, event history.Event, ignoreDuplicate bool) error {
	// don't create duplicate instances
	if !ignoreDuplicate {
		docs, err := b.db.Collection("instances").CountDocuments(sessCtx, bson.M{"instance_id": wfi.InstanceID})
		if err != nil {
			return err
		}
		if docs > 0 {
			return backend.ErrInstanceAlreadyExists
		}
	}

	// metatdata from event that created this instance
	attr, ok := event.Attributes.(*history.ExecutionStartedAttributes)
	if !ok {
		return errors.New("event attributes are not of type ExecutionStartedAttributes")
	}
	md, err := json.Marshal(attr.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	inst := instance{
		InstanceID:  wfi.InstanceID,
		ExecutionID: wfi.ExecutionID,
		Metadata:    md,
		CreatedAt:   time.Now(),
	}
	if wfi.SubWorkflow() {
		inst.ParentInstanceID = &wfi.ParentInstanceID
		inst.ParentScheduleEventID = &wfi.ParentEventID
	}

	if _, err := b.db.Collection("instances").InsertOne(sessCtx, inst); err != nil {
		return fmt.Errorf("inserting workflow instance: %w", err)
	}

	return nil
}

// CancelWorkflowInstance cancels the specified running workflow instance.
func (b *mongoBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	// create transaction
	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// // check we have a running instance
		// count, err := b.db.Collection("instances").CountDocuments(sessCtx, bson.M{"instance_id": instance.InstanceID})
		// if err != nil {
		// 	return nil, err
		// }
		// if count == 0 {
		// 	return nil, backend.ErrInstanceNotFound
		// }

		// add 'cancel' event
		if err := b.insertEvents(sessCtx, "pending_events", instance.InstanceID, []history.Event{*event}); err != nil {
			return nil, fmt.Errorf("inserting cancellation event: %w", err)
		}
		return nil, nil
	}

	// execute
	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

// GetWorkflowInstanceHistory returns history events for the specified workflow instance.
func (b *mongoBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]history.Event, error) {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// find matching events
		filter := bson.M{"instance_id": instance.InstanceID}
		if lastSequenceID != nil {
			filter = bson.M{"$and": bson.A{
				filter,
				bson.M{"sequence_id": bson.M{"$gt": *lastSequenceID}},
			}}
		}
		opts := options.FindOptions{
			Sort: bson.M{"sequence_id": 1},
		}
		cursor, err := b.db.Collection("history").Find(sessCtx, filter, &opts)
		if err != nil {
			return nil, fmt.Errorf("error fetching history: %w", err)
		}
		var evts []event
		if err := cursor.All(sessCtx, &evts); err != nil {
			return nil, err
		}

		// unpack into standard events
		hevts := make([]history.Event, len(evts))
		for i, evt := range evts {
			attr, err := history.DeserializeAttributes(evt.EventType, evt.Attributes)
			if err != nil {
				return nil, fmt.Errorf("deserializing attributes: %w", err)
			}
			hevts[i] = history.Event{
				ID:              evt.EventID,
				SequenceID:      evt.SequenceID,
				Type:            evt.EventType,
				Timestamp:       evt.Timestamp,
				ScheduleEventID: evt.ScheduleEventID,
				VisibleAt:       evt.VisibleAt,
				Attributes:      attr,
			}
		}

		return hevts, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	res, err := session.WithTransaction(ctx, txn)
	if err != nil || res == nil {
		return nil, err
	}

	return res.([]history.Event), nil

}

// GetWorkflowInstanceState returns the state for the specified workflow instance.
func (b *mongoBackend) GetWorkflowInstanceState(ctx context.Context, inst *workflow.Instance) (backend.WorkflowState, error) {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{"$and": bson.A{
			bson.M{"instance_id": inst.InstanceID},
			bson.M{"execution_id": inst.ExecutionID},
		}}

		var i instance
		if err := b.db.Collection("instances").FindOne(sessCtx, filter).Decode(&i); err != nil {
			if err == mongo.ErrNoDocuments {
				return backend.WorkflowStateActive, backend.ErrInstanceNotFound
			}
			return backend.WorkflowStateActive, err
		}

		// CompletedAt set == workflow is finished
		if i.CompletedAt != nil {
			return backend.WorkflowStateFinished, nil
		}

		return backend.WorkflowStateActive, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return backend.WorkflowStateActive, err
	}
	defer session.EndSession(ctx)
	res, err := session.WithTransaction(ctx, txn)
	if err != nil || res == nil {
		return backend.WorkflowStateActive, err
	}

	return res.(backend.WorkflowState), nil
}

// SignalWorkflow signals a running workflow instance
func (b *mongoBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// // check we have an instance
		// count, err := b.db.Collection("instances").CountDocuments(sessCtx, bson.M{"instance_id": instanceID})
		// if err != nil {
		// 	return nil, fmt.Errorf("error counting instances: %w", err)
		// }
		// if count == 0 {
		// 	return nil, backend.ErrInstanceNotFound
		// }

		// add 'signal' event
		if err := b.insertEvents(sessCtx, "pending_events", instanceID, []history.Event{event}); err != nil {
			return nil, fmt.Errorf("inserting signal event: %w", err)
		}

		return nil, nil
	}

	// execute
	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

// GetWorkflowInstance returns a pending workflow task or nil if there are no pending worflow executions
func (b *mongoBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {

	// lock next workflow task by finding an unlocked instance with new events to process.
	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {

		// find an unlocked instance
		filter := bson.M{"$and": bson.A{
			// bson.M{"completed_at": bson.M{"$exists": false}},
			bson.M{"completed_at": nil},
			bson.M{"$or": bson.A{
				bson.M{"locked_until": bson.M{"$lte": time.Now()}},
				// bson.M{"locked_until": bson.M{"$exists": false}},
				bson.M{"locked_until": nil},
			}},
			bson.M{"$or": bson.A{
				bson.M{"sticky_until": bson.M{"$lte": time.Now()}},
				// bson.M{"sticky_until": bson.M{"$exists": false}},
				bson.M{"sticky_until": nil},
				bson.M{"worker": b.workerName},
			}},
		}}

		var inst instance
		if err := b.db.Collection("instances").FindOne(sessCtx, filter).Decode(&inst); err != nil {
			// exit if no instances
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, err
		}

		// check to see if instance has events to process
		count, err := b.db.Collection("pending_events").CountDocuments(sessCtx, bson.M{"instance_id": inst.InstanceID})
		if err != nil {
			return nil, err
		}
		// exit if no pending events to process
		if count == 0 {
			return nil, nil
		}

		// must have an event to process: lock instance
		filter = bson.M{"instance_id": inst.InstanceID}
		upd := bson.D{{Key: "$set", Value: bson.D{
			{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
			{Key: "worker", Value: b.workerName},
		}}}
		if err := b.db.Collection("instances").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			return nil, fmt.Errorf("locking workflow instance: %w", err)
		}

		//
		var wfi *workflow.Instance
		if inst.ParentInstanceID != nil {
			wfi = core.NewSubWorkflowInstance(inst.InstanceID, inst.ExecutionID, *inst.ParentInstanceID, *inst.ParentScheduleEventID)
		} else {
			wfi = core.NewWorkflowInstance(inst.InstanceID, inst.ExecutionID)
		}

		// get new events
		filter = bson.M{"$and": bson.A{
			bson.M{"instance_id": wfi.InstanceID},
			bson.M{"$or": bson.A{
				// bson.M{"visible_at": bson.M{"$exists": false}},
				bson.M{"visible_at": nil},
				bson.M{"visible_at": bson.M{"$lte": time.Now()}},
			}},
		}}
		opts := options.FindOptions{
			// Sort: bson.M{"id": 1},
			Sort: bson.M{"timestamp": 1},
		}
		cursor, err := b.db.Collection("pending_events").Find(sessCtx, filter, &opts)
		if err != nil {
			return nil, fmt.Errorf("getting new events: %w", err)
		}

		var evts []event
		if err := cursor.All(sessCtx, &evts); err != nil {
			return nil, fmt.Errorf("loading new events: %w", err)
		}
		if len(evts) == 0 {
			return nil, nil
		}

		var md *core.WorkflowMetadata
		if err := json.Unmarshal(inst.Metadata, &md); err != nil {
			return nil, fmt.Errorf("parsing workflow metadata: %w", err)
		}

		twf := &task.Workflow{
			ID:               wfi.InstanceID,
			WorkflowInstance: wfi,
			Metadata:         md,
			NewEvents:        make([]history.Event, len(evts)),
		}

		// unpack into standard events
		for i, evt := range evts {
			attr, err := history.DeserializeAttributes(evt.EventType, evt.Attributes)
			if err != nil {
				return nil, fmt.Errorf("deserializing attributes: %w", err)
			}
			twf.NewEvents[i] = history.Event{
				ID:              evt.EventID,
				SequenceID:      evt.SequenceID,
				Type:            evt.EventType,
				Timestamp:       evt.Timestamp,
				ScheduleEventID: evt.ScheduleEventID,
				VisibleAt:       evt.VisibleAt,
				Attributes:      attr,
			}
		}

		// get most recent sequence id
		filter = bson.M{"instance_id": wfi.InstanceID}
		fopts := options.FindOneOptions{Sort: bson.M{"sequence_id": -1}}

		var evt event
		if err := b.db.Collection("history").FindOne(sessCtx, filter, &fopts).Decode(&evt); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, fmt.Errorf("getting most recent sequence id: %w", err)
			}
		}
		twf.LastSequenceID = evt.SequenceID

		return twf, nil
	}

	// execute
	session, err := b.db.Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	res, err := session.WithTransaction(ctx, txn)
	if err != nil || res == nil {
		return nil, err
	}

	return res.(*task.Workflow), nil
}

// CompleteWorkflowTask completes a workflow task retrieved using GetWorkflowTask
//
// This checkpoints the execution. events are new events from the last workflow execution
// which will be added to the workflow instance history. workflowEvents are new events for the
// completed or other workflow instances.
func (b *mongoBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *task.Workflow,
	instance *workflow.Instance,
	state backend.WorkflowState,
	executedEvents, activityEvents, timerEvents []history.Event,
	workflowEvents []history.WorkflowEvent,
) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// unlock instance, but keep it sticky to the current worker
		var completedAt *time.Time
		if state == backend.WorkflowStateFinished {
			t := time.Now()
			completedAt = &t
		}

		filter := bson.M{"$and": bson.A{
			bson.M{"instance_id": instance.InstanceID},
			bson.M{"execution_id": instance.ExecutionID},
			bson.M{"worker": b.workerName},
		}}
		upd := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "locked_until", Value: nil},
				{Key: "sticky_until", Value: time.Now().Add(b.options.StickyTimeout)},
				{Key: "completed_at", Value: completedAt},
			}},
		}
		if err := b.db.Collection("instances").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, errors.New("could not find workflow instance to unlock")
			}
			return nil, fmt.Errorf("unlocking workflow instance: %w", err)
		}

		// remove handled events
		for _, e := range executedEvents {
			filter := bson.M{"$and": bson.A{
				bson.M{"instance_id": instance.InstanceID},
				bson.M{"event_id": e.ID},
			}}
			if _, err := b.db.Collection("pending_events").DeleteOne(sessCtx, filter); err != nil {
				return nil, fmt.Errorf("deleting handled event: %w", err)
			}
		}
		// ... and add to history
		if err := b.insertEvents(sessCtx, "history", instance.InstanceID, executedEvents); err != nil {
			return nil, fmt.Errorf("inserting new history events: %w", err)
		}

		// schedule activities
		for _, e := range activityEvents {
			if err := b.scheduleActivity(sessCtx, instance, e); err != nil {
				return nil, fmt.Errorf("scheduling activity: %w", err)
			}
		}

		// add new timer events
		if err := b.insertEvents(sessCtx, "pending_events", instance.InstanceID, timerEvents); err != nil {
			return nil, fmt.Errorf("scheduling timers: %w", err)
		}
		// ...and remove cancelled ones
		for _, e := range executedEvents {
			switch e.Type {
			case history.EventType_TimerCanceled:
				if err := b.removeFutureEvent(sessCtx, instance.InstanceID, e.ScheduleEventID); err != nil {
					return nil, fmt.Errorf("removing future event: %w", err)
				}
			}
		}

		// insert new workflow events
		groupedEvents := history.EventsByWorkflowInstance(workflowEvents)

		for wfi, events := range groupedEvents {
			for _, event := range events {
				// create instance
				if event.Type == history.EventType_WorkflowExecutionStarted {
					if err := b.createInstance(sessCtx, wfi, event, true); err != nil {
						return nil, err
					}
					break
				}
			}
			// insert event
			if err := b.insertEvents(sessCtx, "pending_events", wfi.InstanceID, events); err != nil {
				return nil, fmt.Errorf("inserting messages: %w", err)
			}
		}

		return nil, nil
	}

	// execute
	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

// ExtendWorkflowTask extends the lock held by this worker on a workflow task.
func (b *mongoBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{"$and": bson.A{
			bson.M{"instance_id": instance.InstanceID},
			bson.M{"execution_id": instance.ExecutionID},
			bson.M{"worker": b.workerName},
		}}
		upd := bson.D{{Key: "$set", Value: bson.D{
			{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
		}}}

		if err := b.db.Collection("instances").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, errors.New("could not find workflow task to extend")
			}
			return nil, fmt.Errorf("error extending workflow task: %w", err)
		}

		return nil, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

// GetActivityTask returns a pending activity task or nil if there are no pending activities
func (b *mongoBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// get next activity
		filter := bson.M{"$or": bson.A{
			bson.M{"locked_until": bson.M{"$exists": false}},
			bson.M{"locked_until": bson.M{"$lt": time.Now()}},
		}}
		var act activity
		if err := b.db.Collection("activities").FindOne(sessCtx, filter).Decode(&act); err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}

		// lock returned activity
		filter = bson.M{"activity_id": act.ActivityID}
		upd := bson.D{{Key: "$set", Value: bson.D{
			{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
			{Key: "worker", Value: b.workerName},
		}}}
		if err := b.db.Collection("activities").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			return nil, fmt.Errorf("locking activity: %w", err)
		}

		// add instance metadata to activity
		filter = bson.M{"instance_id": act.InstanceID}
		var inst instance
		if err := b.db.Collection("instances").FindOne(sessCtx, filter).Decode(&inst); err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, errors.New("was expecting to find a matching instance")
			}
			return nil, fmt.Errorf("getting instance: %w", err)
		}

		var md *workflow.Metadata
		if err := json.Unmarshal(inst.Metadata, &md); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
		attr, err := history.DeserializeAttributes(act.EventType, act.Attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		return &task.Activity{
			ID:               act.ActivityID,
			WorkflowInstance: core.NewWorkflowInstance(act.InstanceID, act.ExecutionID),
			Metadata:         md,
			Event: history.Event{
				ID:              act.ActivityID,
				Type:            act.EventType,
				Timestamp:       act.Timestamp,
				ScheduleEventID: act.ScheduleEventID,
				VisibleAt:       act.VisibleAt,
				Attributes:      attr,
			},
		}, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	res, err := session.WithTransaction(ctx, txn)
	if err != nil || res == nil {
		return nil, err
	}

	return res.(*task.Activity), nil
}

// CompleteActivityTask completes a activity task retrieved using GetActivityTask
func (b *mongoBackend) CompleteActivityTask(ctx context.Context, inst *workflow.Instance, activityID string, event history.Event) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// remove completed activity
		filter := bson.M{"$and": bson.A{
			bson.M{"activity_id": activityID},
			bson.M{"instance_id": inst.InstanceID},
			bson.M{"execution_id": inst.ExecutionID},
			bson.M{"worker": b.workerName},
		}}
		if _, err := b.db.Collection("activities").DeleteOne(sessCtx, filter); err != nil {
			return nil, fmt.Errorf("completing activity: %w", err)
		}

		// insert new event generated during this workflow execution
		if err := b.insertEvents(sessCtx, "pending_events", inst.InstanceID, []history.Event{event}); err != nil {
			return nil, fmt.Errorf("inserting new events for completed activity: %w", err)
		}

		return nil, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

func (b *mongoBackend) ExtendActivityTask(ctx context.Context, activityID string) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{"$and": bson.A{
			bson.M{"activity_id": activityID},
			bson.M{"worker": b.workerName},
		}}
		upd := bson.D{{Key: "$set", Value: bson.D{
			{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
		}}}
		if err := b.db.Collection("activities").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			return nil, fmt.Errorf("extending activity lock: %w", err)
		}

		return nil, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)
	return err
}

func (b *mongoBackend) scheduleActivity(sessCtx mongo.SessionContext, inst *core.WorkflowInstance, event history.Event) error {

	attr, err := history.SerializeAttributes(event.Attributes)
	if err != nil {
		return err
	}

	act := activity{
		ActivityID:      event.ID,
		InstanceID:      inst.InstanceID,
		ExecutionID:     inst.ExecutionID,
		EventType:       event.Type,
		Attributes:      attr,
		Timestamp:       event.Timestamp,
		ScheduleEventID: event.ScheduleEventID,
		VisibleAt:       event.VisibleAt,
	}
	if _, err := b.db.Collection("activities").InsertOne(sessCtx, act); err != nil {
		return fmt.Errorf("inserting activity: %w", err)
	}

	return nil
}
