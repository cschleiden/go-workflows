package mongo

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/history"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func insertEvents(sessCtx mongo.SessionContext, coll *mongo.Collection, instanceID string, events []history.Event) error {

	for _, newEvent := range events {
		a, err := history.SerializeAttributes(newEvent.Attributes)
		if err != nil {
			return err
		}

		evt := event{
			EventID:         newEvent.ID,
			SequenceID:      newEvent.SequenceID,
			InstanceID:      instanceID,
			Type:            newEvent.Type,
			Timestamp:       newEvent.Timestamp,
			ScheduleEventID: newEvent.ScheduleEventID,
			Attributes:      a,
			VisibleAt:       newEvent.VisibleAt,
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
		bson.M{"$set": bson.M{"visible_at": true}},
	}}
	_, err := coll.DeleteOne(context.Background(), filter)
	return err
}
