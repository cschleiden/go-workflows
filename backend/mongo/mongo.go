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
	"go.opentelemetry.io/otel/trace"
)

type mongoBackend struct {
	db         *mongo.Database
	workerName string
	options    backend.Options
}

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
func (b *mongoBackend) Logger() log.Logger {
	return b.options.Logger
}

func (b *mongoBackend) Tracer() trace.Tracer {
	return b.options.TracerProvider.Tracer(backend.TracerName)
}

func NewMongoBackend(uri, app string, opts ...backend.BackendOption) (*mongoBackend, error) {
	// for _, opt := range opts {
	// 	opt(&c)
	// }

	// connect to db
	client, err := mongo.Connect(
		context.Background(),
		options.Client().ApplyURI(uri),
		options.Client().SetAppName(app),
		options.Client().SetConnectTimeout(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to mongo: %w", err)
	}

	return &mongoBackend{
		db:         client.Database(app),
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
		options:    backend.ApplyOptions(opts...),
	}, nil
}

// CreateWorkflowInstance creates a new workflow instance
func (b *mongoBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event history.Event) error {
	// create transaction
	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// create workflow instance
		attr, ok := event.Attributes.(*history.ExecutionStartedAttributes)
		if !ok {
			return nil, errors.New("event attributes are not of type ExecutionStartedAttributes")
		}

		coll := b.db.Collection("instances")
		if err := createInstance(sessCtx, coll, instance, attr.Metadata, false); err != nil {
			return nil, err
		}

		// initial history is empty, add new event
		coll = b.db.Collection("pending_events")
		if err := insertEvents(sessCtx, coll, instance.InstanceID, []history.Event{event}); err != nil {
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

func createInstance(sessCtx mongo.SessionContext, coll *mongo.Collection, wfi *workflow.Instance, metadata *workflow.Metadata, ignoreDuplicate bool) error {
	// don't create duplicate instances
	if !ignoreDuplicate {
		docs, err := coll.CountDocuments(sessCtx, bson.M{"instance_id": wfi.InstanceID})
		if err != nil {
			return err
		}
		if docs > 0 {
			return backend.ErrInstanceAlreadyExists
		}
	}

	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	inst := instance{
		InstanceID:  wfi.InstanceID,
		ExecutionID: wfi.ExecutionID,
		Metadata:    metadataJson,
		CreatedAt:   time.Now(),
	}
	if wfi.SubWorkflow() {
		inst.ParentInstanceID = &wfi.ParentInstanceID
		inst.ParentScheduleEventID = &wfi.ParentEventID
	}

	if _, err := coll.InsertOne(sessCtx, inst); err != nil {
		return fmt.Errorf("inserting workflow instance: %w", err)
	}

	return nil
}

// CancelWorkflowInstance cancels a running workflow instance
func (b *mongoBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	// create transaction
	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// check we have a running instance
		// TODO: Combine this with the event insertion
		count, err := b.db.Collection("instances").CountDocuments(sessCtx, bson.M{"instance_id": instance.InstanceID})
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, backend.ErrInstanceNotFound
		}

		// add 'cancel' event
		coll := b.db.Collection("pending_events")
		if err := insertEvents(sessCtx, coll, instance.InstanceID, []history.Event{*event}); err != nil {
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

func (b *mongoBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]history.Event, error) {
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
	cursor, err := b.db.Collection("history").Find(ctx, filter, &opts)
	if err != nil {
		return nil, err
	}
	var evts []event
	if err := cursor.All(ctx, &evts); err != nil {
		return nil, err
	}

	// unpack into standard events
	hevts := make([]history.Event, len(evts))
	for i := 0; i < len(evts); i++ {
		attr, err := history.DeserializeAttributes(hevts[i].Type, evts[i].Attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}
		hevts[i] = history.Event{
			ID:              evts[i].EventID,
			SequenceID:      evts[i].SequenceID,
			Type:            evts[i].Type,
			Timestamp:       evts[i].Timestamp,
			ScheduleEventID: evts[i].ScheduleEventID,
			VisibleAt:       evts[i].VisibleAt,
			Attributes:      attr,
		}
	}

	return hevts, nil
}

func (b *mongoBackend) GetWorkflowInstanceState(ctx context.Context, inst *workflow.Instance) (backend.WorkflowState, error) {
	filter := bson.M{"$and": bson.A{
		bson.M{"instance_id": inst.InstanceID},
		bson.M{"execution_id": inst.ExecutionID},
	}}

	var i instance
	if err := b.db.Collection("instances").FindOne(ctx, filter).Decode(&i); err != nil {
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

// SignalWorkflow signals a running workflow instance
func (b *mongoBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// check we have an instance
		// TODO: Combine this with the event insertion
		count, err := b.db.Collection("instances").CountDocuments(sessCtx, bson.M{"instance_id": instanceID})
		if err != nil {
			return nil, fmt.Errorf("error counting instances: %w", err)
		}
		if count == 0 {
			return nil, backend.ErrInstanceNotFound
		}

		// add 'cancel' event
		coll := b.db.Collection("pending_events")
		if err := insertEvents(sessCtx, coll, instanceID, []history.Event{event}); err != nil {
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

	var twf *task.Workflow

	// lock next workflow task by finding an unlocked instance with new events to process.
	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// find an unlocked instance with events to process
				pl := bson.A{
			// find matching instances
			bson.D{{Key: "$match", Value: bson.D{{Key: "$and", Value: bson.A{
				bson.M{"completed_at": bson.M{"$exists": false}},
				bson.M{"$or": bson.A{
					bson.M{"locked_until": bson.M{"$exists": false}},
					bson.M{"locked_until": bson.M{"$lt": time.Now()}},
				}},
				bson.M{"$or": bson.A{
					bson.M{"sticky_until": bson.M{"$exists": false}},
					bson.M{"sticky_until": bson.M{"$lt": time.Now()}},
					bson.M{"worker": bson.M{"$eq": b.workerName}},
				}},
			}}}}},
			// find pending_events ready for processing
			bson.D{{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "pending_events"},
				{Key: "localField", Value: "instance_id"},
				{Key: "foreignField", Value: "instance_id"},
				{Key: "pipeline", Value: bson.A{
					bson.D{{Key: "$match",
						Value: bson.D{{Key: "$or", Value: bson.A{
							bson.M{"visible_at": bson.M{"$exists": false}},
							bson.M{"visible_at": bson.M{"$lte": time.Now()}},
						}}},
					}},
					bson.D{{Key: "$project", Value: bson.D{{Key: "instance_id", Value: 1}}}},
				}},
				{Key: "as", Value: "pending_events"},
			}}},
		}

		cursor, err := b.db.Collection("instances").Aggregate(sessCtx, pl)
		if err != nil {
			return nil, err
		}
		var matches []struct {
			instance
			PendingEvents []string `bson:"pending_events"`
		}
		if err := cursor.All(sessCtx, &matches); err != nil {
			return nil, fmt.Errorf("error loading workflow instance(s): %w", err)
		}

		// we are expecting a single match
		if len(matches) != 1 {
			return nil, fmt.Errorf("expecting 1 instance but got %d", len(matches))
		}
		res := matches[0]

		// no new events to process, so we can exit
		if len(res.PendingEvents) == 0 {
			return nil, nil
		}

		// lock instance
		filter := bson.M{"instance_id": res.InstanceID}
		upd := bson.D{{Key: "$set", Value: bson.D{
			{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
			{Key: "worker", Value: b.workerName},
		}}}
		if err := b.db.Collection("instances").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			return nil, fmt.Errorf("locking workflow instance: %w", err)
		}

		//
		var wfi *workflow.Instance
		if res.ParentInstanceID != nil {
			wfi = core.NewSubWorkflowInstance(res.InstanceID, res.ExecutionID, *res.ParentInstanceID, *res.ParentScheduleEventID)
		} else {
			wfi = core.NewWorkflowInstance(res.InstanceID, res.ExecutionID)
		}

		var metadata *core.WorkflowMetadata
		if err := json.Unmarshal(res.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("parsing workflow metadata: %w", err)
		}

		// get new events
		filter = bson.M{"$and": bson.A{
			bson.M{"instance_id": wfi.InstanceID},
			bson.M{"$or": bson.A{
				bson.M{"$set": bson.M{"visible_at": false}},
				bson.M{"visible_at": bson.M{"$lte": time.Now()}},
			}},
		}}
		opts := options.FindOptions{
			Sort: bson.M{"id": 1},
		}

		cursor, err = b.db.Collection("pending_events").Find(sessCtx, filter, &opts)
		if err != nil {
			return nil, fmt.Errorf("getting new events: %w", err)
		}
		var evts []event
		if err := cursor.All(sessCtx, &evts); err != nil {
			return nil, fmt.Errorf("loading new events: %w", err)
		}

		// no new events to process, so we can exit
		if len(evts) == 0 {
			return nil, nil
		}

		twf = &task.Workflow{
			ID:               wfi.InstanceID,
			WorkflowInstance: wfi,
			Metadata:         metadata,
			NewEvents:        make([]history.Event, len(evts)),
		}

		// unpack into standard events
		for i := 0; i < len(evts); i++ {
			twf.NewEvents[i].ID = evts[i].EventID
			twf.NewEvents[i].SequenceID = evts[i].SequenceID
			twf.NewEvents[i].Type = evts[i].Type
			twf.NewEvents[i].Timestamp = evts[i].Timestamp
			twf.NewEvents[i].ScheduleEventID = evts[i].ScheduleEventID
			twf.NewEvents[i].VisibleAt = evts[i].VisibleAt

			a, err := history.DeserializeAttributes(twf.NewEvents[i].Type, evts[i].Attributes)
			if err != nil {
				return nil, fmt.Errorf("deserializing attributes: %w", err)
			}
			twf.NewEvents[i].Attributes = a
		}

		// Get most recent sequence id
		filter = bson.M{"instance_id": wfi.InstanceID}
		fopts := options.FindOneOptions{Sort: bson.M{"id": -1}}
		var evt event
		if err := b.db.Collection("history").FindOne(sessCtx, filter, &fopts).Decode(&evt); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, errors.New("was expecting to find an event")
			}
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}
		twf.LastSequenceID = evt.SequenceID

		return nil, nil
	}

	// execute
	session, err := b.db.Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	if _, err := session.WithTransaction(ctx, txn); err != nil {
		return nil, err
	}

	return twf, nil
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

		// insert new events generated during this workflow execution to the history
		coll := b.db.Collection("history")
		if err := insertEvents(sessCtx, coll, instance.InstanceID, executedEvents); err != nil {
			return nil, fmt.Errorf("inserting new history events: %w", err)
		}

		// Schedule activities
		coll = b.db.Collection("activities")
		for _, e := range activityEvents {
			if err := scheduleActivity(sessCtx, coll, instance, e); err != nil {
				return nil, fmt.Errorf("scheduling activity: %w", err)
			}
		}

		// timer events
		coll = b.db.Collection("pending_events")
		if err := insertEvents(sessCtx, coll, instance.InstanceID, timerEvents); err != nil {
			return nil, fmt.Errorf("scheduling timers: %w", err)
		}

		for _, event := range executedEvents {
			switch event.Type {
			case history.EventType_TimerCanceled:
				coll := b.db.Collection("pending_events")
				if err := removeFutureEvent(sessCtx, coll, instance.InstanceID, event.ScheduleEventID); err != nil {
					return nil, fmt.Errorf("removing future event: %w", err)
				}
			}
		}

		// Insert new workflow events
		groupedEvents := history.EventsByWorkflowInstance(workflowEvents)

		for targetInstance, events := range groupedEvents {
			for _, event := range events {
				// create instance
				if event.Type == history.EventType_WorkflowExecutionStarted {
					a := event.Attributes.(*history.ExecutionStartedAttributes)
					coll := b.db.Collection("instances")
					if err := createInstance(sessCtx, coll, targetInstance, a.Metadata, true); err != nil {
						return nil, err
					}
					break
				}
			}
			// insert event
			coll := b.db.Collection("pending_events")
			if err := insertEvents(sessCtx, coll, targetInstance.InstanceID, events); err != nil {
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

func (b *mongoBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {

	filter := bson.M{"$and": bson.A{
		bson.M{"instance_id": instance.InstanceID},
		bson.M{"execution_id": instance.ExecutionID},
		bson.M{"worker": b.workerName},
	}}
	upd := bson.D{{Key: "$set", Value: bson.D{
		{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
	}}}

	if err := b.db.Collection("instances").FindOneAndUpdate(ctx, filter, upd).Err(); err != nil {
		if err != mongo.ErrNoDocuments {
			return errors.New("could not find workflow task to extend")
		}
		return fmt.Errorf("error extending workflow task: %w", err)
	}

	return nil
}

// GetActivityTask returns a pending activity task or nil if there are no pending activities
func (b *mongoBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {

	var tsk *task.Activity

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// get next activity
		filter := bson.M{"$or": bson.A{
			bson.M{"$set": bson.M{"locked_until": false}},
			bson.M{"locked_until": bson.M{"$lt": time.Now()}},
		}}
		var act activity
		if err := b.db.Collection("activities").FindOne(sessCtx, filter).Decode(&act); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, errors.New("was expecting to find an activity")
			}
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}

		// get instance metadata
		filter = bson.M{"instance_id": act.InstanceID}
		var inst instance
		if err := b.db.Collection("instances").FindOne(sessCtx, filter).Decode(&inst); err != nil {
			if err != mongo.ErrNoDocuments {
				return nil, errors.New("was expecting to find a matching instance")
			}
			return nil, fmt.Errorf("getting instance: %w", err)
		}

		var metadata *workflow.Metadata
		if err := json.Unmarshal(inst.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
		a, err := history.DeserializeAttributes(act.EventType, act.Attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		evt := history.Event{
			ID:              act.ActivityID,
			Type:            act.EventType,
			Timestamp:       act.Timestamp,
			ScheduleEventID: act.ScheduleEventID,
			VisibleAt:       act.VisibleAt,
			Attributes:      a,
		}

		filter = bson.M{"id": act.ID}
		upd := bson.D{{Key: "$set", Value: bson.D{
			{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
			{Key: "worker", Value: b.workerName},
		}}}
		if err := b.db.Collection("instances").FindOneAndUpdate(sessCtx, filter, upd).Err(); err != nil {
			return nil, fmt.Errorf("locking workflow instance: %w", err)
		}

		tsk = &task.Activity{
			ID:               act.ActivityID,
			WorkflowInstance: core.NewWorkflowInstance(act.InstanceID, act.ExecutionID),
			Metadata:         metadata,
			Event:            evt,
		}

		return nil, nil
	}

	session, err := b.db.Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, txn)

	return tsk, nil
}

// CompleteActivityTask completes a activity task retrieved using GetActivityTask
func (b *mongoBackend) CompleteActivityTask(ctx context.Context, instance *workflow.Instance, id string, event history.Event) error {

	txn := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// remove completed activity
		filter := bson.M{"$and": bson.A{
			bson.M{"activity_id": id},
			bson.M{"instance_id": instance.InstanceID},
			bson.M{"execution_id": instance.ExecutionID},
			bson.M{"worker": b.workerName},
		}}
		if _, err := b.db.Collection("pending_events").DeleteOne(sessCtx, filter); err != nil {
			return nil, fmt.Errorf("completing activity: %w", err)
		}

		// insert new event generated during this workflow execution
		coll := b.db.Collection("pending_events")
		if err := insertEvents(sessCtx, coll, instance.InstanceID, []history.Event{event}); err != nil {
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

	filter := bson.M{"$and": bson.A{
		bson.M{"activity_id": activityID},
		bson.M{"worker": b.workerName},
	}}
	upd := bson.D{{Key: "$set", Value: bson.D{
		{Key: "locked_until", Value: time.Now().Add(b.options.WorkflowLockTimeout)},
	}}}
	if err := b.db.Collection("activities").FindOneAndUpdate(ctx, filter, upd).Err(); err != nil {
		return fmt.Errorf("extending activity lock: %w", err)
	}

	return nil
}

func scheduleActivity(sessCtx mongo.SessionContext, coll *mongo.Collection, inst *core.WorkflowInstance, event history.Event) error {

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
	if _, err := coll.InsertOne(sessCtx, act); err != nil {
		return fmt.Errorf("inserting activity: %w", err)
	}

	return err
}
