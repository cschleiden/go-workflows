package history

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

type EventType uint

const (
	_ EventType = iota

	EventType_WorkflowExecutionStarted
	EventType_WorkflowExecutionFinished
	EventType_WorkflowExecutionTerminated
	EventType_WorkflowExecutionCancelled

	EventType_WorkflowTaskStarted
	EventType_WorkflowTaskFinished

	EventType_SubWorkflowScheduled
	EventType_SubWorkflowCompleted
	EventType_SubWorkflowFailed

	EventType_ActivityScheduled
	EventType_ActivityCompleted
	EventType_ActivityFailed

	EventType_TimerScheduled
	EventType_TimerFired

	EventType_SignalReceived
)

type Event struct {
	// ID is a unique identifier
	ID string

	Type EventType

	Timestamp time.Time

	// EventID is used to correlate events belonging together
	// For example, if an activity is scheduled, EventID of the schedule event and the
	// completion/failure event are the same.
	EventID int

	// Attributes are event type specific attributes
	Attributes interface{}

	VisibleAt *time.Time
}

func (e *Event) String() string {
	return strconv.Itoa(int(e.Type))
}

func NewHistoryEvent(eventType EventType, eventID int, attributes interface{}) Event {
	return Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		Timestamp:  time.Now().UTC(),
		EventID:    eventID,
		Attributes: attributes,
	}
}

func NewFutureHistoryEvent(eventType EventType, eventID int, attributes interface{}, visibleAt time.Time) Event {
	event := NewHistoryEvent(eventType, eventID, attributes)
	event.VisibleAt = &visibleAt
	return event
}
