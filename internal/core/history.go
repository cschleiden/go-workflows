package core

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

type HistoryEntry struct {
	Type HistoryEventType
}
