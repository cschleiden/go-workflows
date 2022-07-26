package mongo

import (
	"fmt"

	"github.com/ticctech/go-workflows/internal/history"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (b *mongoBackend) insertEvents(sessCtx mongo.SessionContext, collName, instanceID string, events []history.Event) error {
	if len(events) == 0 {
		return nil
	}
	
	coll := b.db.Collection(collName)
	evts := make([]interface{}, len(events))

	for i, e := range events {
		attr, err := history.SerializeAttributes(e.Attributes)
		if err != nil {
			return err
		}
		evts[i] = event{
			EventID:         e.ID,
			InstanceID:      instanceID,
			SequenceID:      e.SequenceID,
			Type:            e.Type,
			Timestamp:       e.Timestamp,
			ScheduleEventID: e.ScheduleEventID,
			Attributes:      attr,
			VisibleAt:       e.VisibleAt,
		}
	}

	if _, err := coll.InsertMany(sessCtx, evts); err != nil {
		return fmt.Errorf("error inserting events: %w", err)
	}

	return nil
}

func (b *mongoBackend) removeFutureEvent(sessCtx mongo.SessionContext, instanceID string, scheduleEventID int64) error {
	filter := bson.M{"$and": bson.A{
		bson.M{"instance_id": instanceID},
		bson.M{"schedule_event_id": scheduleEventID},
		bson.M{"visible_at": bson.M{"$exists": true}},
	}}
	_, err := b.db.Collection("pending_events").DeleteOne(sessCtx, filter)
	return err
}
