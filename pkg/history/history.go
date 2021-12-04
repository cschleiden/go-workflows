package history

type HistoryEventType uint

const (
	HistoryEventTypeNone HistoryEventType = iota

	HistoryEventTypeWorkflowStarted
	HistoryEventTypeWorkflowFinished

	HistoryEventTypeWorkflowExecutionStarted
	HistoryEventTypeWorkflowExecutionFinished

	HistoryEventTypeActivityScheduled
	HistoryEventTypeActivityStarted
	HistoryEventTypeActivityCompleted
)

type HistoryEvent struct {
	Type HistoryEventType

	EventID int64

	// Attributes are event type specific attributes
	Attributes interface{}
}
