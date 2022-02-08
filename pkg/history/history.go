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

	EventType_SideEffectResult
)

func (et EventType) String() string {
	switch et {
	case EventType_WorkflowExecutionStarted:
		return "WorkflowExecutionStarted"
	case EventType_WorkflowExecutionFinished:
		return "WorkflowExecutionFinished"
	case EventType_WorkflowExecutionTerminated:
		return "WorkflowExecutionTerminated"
	case EventType_WorkflowExecutionCancelled:
		return "WorkflowExecutionCancelled"
	case EventType_WorkflowTaskStarted:
		return "WorkflowTaskStarted"
	case EventType_WorkflowTaskFinished:
		return "WorkflowTaskFinished"
	case EventType_SubWorkflowScheduled:
		return "SubWorkflowScheduled"
	case EventType_SubWorkflowCompleted:
		return "SubWorkflowCompleted"
	case EventType_SubWorkflowFailed:
		return "SubWorkflowFailed"
	case EventType_ActivityScheduled:
		return "ActivityScheduled"
	case EventType_ActivityCompleted:
		return "ActivityCompleted"
	case EventType_ActivityFailed:
		return "ActivityFailed"
	case EventType_TimerScheduled:
		return "TimerScheduled"
	case EventType_TimerFired:
		return "TimerFired"
	case EventType_SignalReceived:
		return "SignalReceived"
	case EventType_SideEffectResult:
		return "SideEffectResult"
	default:
		return "Unknown"
	}
}

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

func (e Event) String() string {
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

func NewWorkflowCancellationEvent() Event {
	return NewHistoryEvent(EventType_WorkflowExecutionCancelled, -1, &ExecutionCancelledAttributes{})
}
