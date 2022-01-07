package history

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

type HistoryEventType uint

const (
	_ HistoryEventType = iota

	HistoryEventType_OrchestratorStarted
	HistoryEventType_OrchestratorFinished

	HistoryEventType_WorkflowExecutionStarted
	HistoryEventType_WorkflowExecutionFinished
	HistoryEventType_WorkflowExecutionFailed
	HistoryEventType_WorkflowExecutionTerminated

	HistoryEventType_ActivityScheduled
	HistoryEventType_ActivityCompleted
	HistoryEventType_ActivityFailed

	HistoryEventType_TimerScheduled
	HistoryEventType_TimerFired

	HistoryEventType_SignalReceived
)

type HistoryEvent struct {
	// ID is a unique identifier
	ID string

	EventType HistoryEventType

	// EventID is a sequence number
	EventID int

	// Attributes are event type specific attributes
	Attributes interface{}

	VisibleAt *time.Time
}

func (e *HistoryEvent) String() string {
	return strconv.Itoa(int(e.EventType))
}

func NewHistoryEvent(eventType HistoryEventType, eventID int, attributes interface{}) HistoryEvent {
	return HistoryEvent{
		ID:         uuid.NewString(),
		EventType:  eventType,
		EventID:    eventID,
		Attributes: attributes,
	}
}
