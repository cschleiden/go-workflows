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
	EventType_WorkflowExecutionCanceled

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
	case EventType_WorkflowExecutionCanceled:
		return "WorkflowExecutionCanceled"

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
	// ID is a unique identifier for this event
	ID string

	Type EventType

	Timestamp time.Time

	SequenceID int64

	// ScheduleEventID is used to correlate events belonging together
	// For example, if an activity is scheduled, ScheduleEventID of the schedule event and the
	// completion/failure event are the same.
	ScheduleEventID int64

	// Attributes are event type specific attributes
	Attributes interface{}

	VisibleAt *time.Time
}

func (e Event) String() string {
	return strconv.Itoa(int(e.Type))
}

type HistoryEventOption func(e *Event)

func ScheduleEventID(scheduleEventID int64) HistoryEventOption {
	return func(e *Event) {
		e.ScheduleEventID = scheduleEventID
	}
}

func VisibleAt(visibleAt time.Time) HistoryEventOption {
	return func(e *Event) {
		e.VisibleAt = &visibleAt
	}
}

func NewHistoryEvent(timestamp time.Time, eventType EventType, attributes interface{}, opts ...HistoryEventOption) Event {
	e := Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		Timestamp:  timestamp,
		Attributes: attributes,
	}

	for _, opt := range opts {
		opt(&e)
	}

	return e
}

func NewWorkflowCancellationEvent(timestamp time.Time) Event {
	return NewHistoryEvent(timestamp, EventType_WorkflowExecutionCanceled, &ExecutionCanceledAttributes{})
}
