package history

import (
	"encoding/json"
	"errors"
)

func SerializeAttributes(attributes interface{}) ([]byte, error) {
	return json.Marshal(attributes)
}

func DeserializeAttributes(eventType EventType, attributes []byte) (attr interface{}, err error) {
	switch eventType {
	case EventType_WorkflowExecutionStarted:
		attr = &ExecutionStartedAttributes{}
	case EventType_WorkflowExecutionFinished:
		attr = &ExecutionCompletedAttributes{}

	case EventType_WorkflowTaskStarted:
		attr = &WorkflowTaskStartedAttributes{}
	case EventType_WorkflowTaskFinished:
		attr = &WorkflowTaskFinishedAttributes{}

	case EventType_ActivityScheduled:
		attr = &ActivityScheduledAttributes{}
	case EventType_ActivityCompleted:
		attr = &ActivityCompletedAttributes{}
	case EventType_ActivityFailed:
		attr = &ActivityFailedAttributes{}

	case EventType_SignalReceived:
		attr = &SignalReceivedAttributes{}

	case EventType_TimerScheduled:
		attr = &TimerScheduledAttributes{}
	case EventType_TimerFired:
		attr = &TimerFiredAttributes{}

	case EventType_SubWorkflowScheduled:
		attr = &SubWorkflowScheduledAttributes{}
	case EventType_SubWorkflowCompleted:
		attr = &SubWorkflowCompletedAttributes{}

	default:
		return nil, errors.New("unknown event type when deserializing attributes")
	}

	err = json.Unmarshal([]byte(attributes), &attr)
	return attr, err
}
