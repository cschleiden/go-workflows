package mongo

import (
	"github.com/ticctech/go-workflows/internal/history"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func insertEvents(sessCtx mongo.SessionContext, coll *mongo.Collection, instanceID string, events []history.Event) error {

	for _, e := range events {
		attr, err := history.SerializeAttributes(e.Attributes)
		if err != nil {
			return err
		}

		evt := event{
			EventID:         e.ID,
			SequenceID:      e.SequenceID,
			InstanceID:      instanceID,
			Type:            e.Type,
			Timestamp:       e.Timestamp,
			ScheduleEventID: e.ScheduleEventID,
			Attributes:      attr,
			VisibleAt:       e.VisibleAt,
		}

		if _, err := coll.InsertOne(sessCtx, evt); err != nil {
			return err
		}
	}

	return nil
}

func removeFutureEvent(sessCtx mongo.SessionContext, coll *mongo.Collection, instanceID string, scheduleEventID int64) error {
	filter := bson.M{"$and": bson.A{
		bson.M{"instance_id": instanceID},
		bson.M{"schedule_event_id": scheduleEventID},
		bson.M{"visible_at": bson.M{"$exists": true}},
	}}
	_, err := coll.DeleteOne(sessCtx, filter)
	return err
}
