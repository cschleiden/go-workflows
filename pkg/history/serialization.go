package history

import (
	"encoding/json"
	"errors"
)

func SerializeAttributes(attributes interface{}) ([]byte, error) {
	return json.Marshal(attributes)
}

func DeserializeAttributes(eventType HistoryEventType, attributes []byte) (attr interface{}, err error) {
	switch eventType {
	case HistoryEventType_WorkflowExecutionStarted:
		attr = &ExecutionStartedAttributes{}
	case HistoryEventType_WorkflowExecutionFinished:
		attr = &ExecutionCompletedAttributes{}

	case HistoryEventType_ActivityScheduled:
		attr = &ActivityScheduledAttributes{}
	case HistoryEventType_ActivityCompleted:
		attr = &ActivityCompletedAttributes{}

	case HistoryEventType_SignalReceived:
		attr = &SignalReceivedAttributes{}

	case HistoryEventType_TimerScheduled:
		attr = &TimerScheduledAttributes{}
	case HistoryEventType_TimerFired:
		attr = &TimerFiredAttributes{}

	case HistoryEventType_SubWorkflowScheduled:
		attr = &SubWorkflowScheduledAttributes{}
	case HistoryEventType_SubWorkflowCompleted:
		attr = &SubWorkflowCompletedAttributes{}

	default:
		return nil, errors.New("unknown event type when deserializing attributes")
	}

	err = json.Unmarshal([]byte(attributes), &attr)
	return attr, err
}
