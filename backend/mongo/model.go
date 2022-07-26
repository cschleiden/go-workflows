package mongo

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/history"
)

type instance struct {
	InstanceID            string     `bson:"instance_id"`
	ExecutionID           string     `bson:"execution_id"`
	ParentInstanceID      *string    `bson:"parent_instance_id"`
	ParentScheduleEventID *int64     `bson:"parent_schedule_event_id"`
	Metadata              []byte     `bson:"metadata"`
	CreatedAt             time.Time  `bson:"created_at"`
	CompletedAt           *time.Time `bson:"completed_at"`
	LockedUntil           *time.Time `bson:"locked_until"`
	StickyUntil           *time.Time `bson:"sticky_until"`
	Worker                *string    `bson:"worker"`
}

type activity struct {
	ID              string            `bson:"id"`
	ActivityID      string            `bson:"activity_id"`
	InstanceID      string            `bson:"instance_id"`
	ExecutionID     string            `bson:"execution_id"`
	EventType       history.EventType `bson:"event_type"`
	Timestamp       time.Time         `bson:"timestamp"`
	ScheduleEventID int64             `bson:"schedule_event_id"`
	Attributes      []byte            `bson:"attributes"`
	VisibleAt       *time.Time        `bson:"visible_at"`
	LockedUntil     *time.Time        `bson:"locked_until"`
	Worker          *string           `bson:"worker"`
}

type event struct {
	EventID         string            `bson:"event_id"`
	SequenceID      int64             `bson:"sequence_id"`
	InstanceID      string            `bson:"instance_id"`
	Type            history.EventType `bson:"event_type"`
	Timestamp       time.Time         `bson:"timestamp"`
	ScheduleEventID int64             `bson:"schedule_event_id"`
	Attributes      []byte            `bson:"attributes"`
	VisibleAt       *time.Time        `bson:"visible_at"`
}
