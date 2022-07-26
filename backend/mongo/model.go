package mongo

import (
	"time"

	"github.com/ticctech/go-workflows/internal/history"
)

type instance struct {
	InstanceID            string     `bson:"instance_id"`
	ExecutionID           string     `bson:"execution_id"`
	ParentInstanceID      *string    `bson:"parent_instance_id,omitempty"`
	ParentScheduleEventID *int64     `bson:"parent_schedule_event_id,omitempty"`
	Metadata              []byte     `bson:"metadata"`
	CreatedAt             time.Time  `bson:"created_at"`
	CompletedAt           *time.Time `bson:"completed_at,omitempty"`
	LockedUntil           *time.Time `bson:"locked_until,omitempty"`
	StickyUntil           *time.Time `bson:"sticky_until,omitempty"`
	Worker                *string    `bson:"worker,omitempty"`
}

type activity struct {
	ActivityID      string            `bson:"activity_id"`
	InstanceID      string            `bson:"instance_id"`
	ExecutionID     string            `bson:"execution_id"`
	EventType       history.EventType `bson:"event_type"`
	Timestamp       time.Time         `bson:"timestamp"`
	ScheduleEventID int64             `bson:"schedule_event_id"`
	Attributes      []byte            `bson:"attributes"`
	VisibleAt       *time.Time        `bson:"visible_at,omitempty"`
	LockedUntil     *time.Time        `bson:"locked_until,omitempty"`
	Worker          *string           `bson:"worker,omitempty"`
}

type event struct {
	EventID         string            `bson:"event_id"`
	InstanceID      string            `bson:"instance_id"`
	SequenceID      int64             `bson:"sequence_id"`
	EventType       history.EventType `bson:"event_type"`
	Timestamp       time.Time         `bson:"timestamp"`
	ScheduleEventID int64             `bson:"schedule_event_id"`
	Attributes      []byte            `bson:"attributes"`
	VisibleAt       *time.Time        `bson:"visible_at,omitempty"`
}
