package history

type HistoryEventType uint

const (
	HistoryEventTypeNone HistoryEventType = iota

	// TODO: Needed?
	// HistoryEventType_WorkflowStarted
	// HistoryEventType_WorkflowFinished

	HistoryEventType_WorkflowExecutionStarted
	HistoryEventType_WorkflowExecutionFinished
	HistoryEventType_WorkflowExecutionFailed
	HistoryEventType_WorkflowExecutionTerminated

	HistoryEventType_ActivityScheduled
	HistoryEventType_ActivityCompleted
	HistoryEventType_ActivityFailed

	// TODO: Add other types
)

type HistoryEvent struct {
	Type HistoryEventType

	EventID int64

	// Attributes are event type specific attributes
	Attributes interface{}
}

func NewHistoryEvent(eventType HistoryEventType, eventID int64, attributes interface{}) HistoryEvent {
	return HistoryEvent{
		Type:       eventType,
		EventID:    eventID,
		Attributes: attributes,
	}
}
