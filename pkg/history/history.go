package history

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

type EventType uint

const (
	_ EventType = iota

	EventType_OrchestratorStarted
	EventType_OrchestratorFinished

	EventType_WorkflowExecutionStarted
	EventType_WorkflowExecutionFinished
	EventType_WorkflowExecutionTerminated

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

	EventType EventType

	// EventID is a sequence number
	EventID int

	// Attributes are event type specific attributes
	Attributes interface{}

	VisibleAt *time.Time
}

func (e *Event) String() string {
	return strconv.Itoa(int(e.EventType))
}

func NewHistoryEvent(eventType EventType, eventID int, attributes interface{}) Event {
	return Event{
		ID:         uuid.NewString(),
		EventType:  eventType,
		EventID:    eventID,
		Attributes: attributes,
	}
}
