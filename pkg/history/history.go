package history

type HistoryEventType uint

const (
	HistoryEventTypeNone HistoryEventType = iota

	HistoryEventType_OrchestratorStarted
	HistoryEventType_OrchestratorFinished

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
	EventType HistoryEventType

	EventID int

	// Attributes are event type specific attributes
	Attributes interface{}

	Played bool
}

func NewHistoryEvent(eventType HistoryEventType, eventID int, attributes interface{}) HistoryEvent {
	return HistoryEvent{
		EventType:  eventType,
		EventID:    eventID,
		Attributes: attributes,
	}
}
