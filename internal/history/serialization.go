package history

import (
	"encoding/json"
	"errors"
)

func (e *Event) UnmarshalJSON(data []byte) error {
	type Aevent Event
	a := &struct {
		// Attributes allows us to defer unmarshaling the events. Has to match the struct tag in Event
		Attributes json.RawMessage `json:"attr,omitempty"`
		*Aevent
	}{}

	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	*e = *(*Event)(a.Aevent)
	attributes, err := DeserializeAttributes(e.Type, a.Attributes)
	if err != nil {
		return err
	}

	e.Attributes = attributes

	return nil
}

func SerializeAttributes(attributes interface{}) ([]byte, error) {
	return json.Marshal(attributes)
}

func DeserializeAttributes(eventType EventType, attributes []byte) (attr interface{}, err error) {
	switch eventType {
	case EventType_WorkflowExecutionStarted:
		attr = &ExecutionStartedAttributes{}
	case EventType_WorkflowExecutionFinished:
		attr = &ExecutionCompletedAttributes{}
	case EventType_WorkflowExecutionCanceled:
		attr = &ExecutionCanceledAttributes{}

	case EventType_WorkflowTaskStarted:
		attr = &WorkflowTaskStartedAttributes{}

	case EventType_ActivityScheduled:
		attr = &ActivityScheduledAttributes{}
	case EventType_ActivityCompleted:
		attr = &ActivityCompletedAttributes{}
	case EventType_ActivityFailed:
		attr = &ActivityFailedAttributes{}

	case EventType_SignalReceived:
		attr = &SignalReceivedAttributes{}

	case EventType_SideEffectResult:
		attr = &SideEffectResultAttributes{}

	case EventType_TimerScheduled:
		attr = &TimerScheduledAttributes{}
	case EventType_TimerFired:
		attr = &TimerFiredAttributes{}
	case EventType_TimerCanceled:
		attr = &TimerCanceledAttributes{}

	case EventType_SubWorkflowScheduled:
		attr = &SubWorkflowScheduledAttributes{}
	case EventType_SubWorkflowCancellationRequested:
		attr = &SubWorkflowCancellationRequestedAttributes{}
	case EventType_SubWorkflowCompleted:
		attr = &SubWorkflowCompletedAttributes{}
	case EventType_SubWorkflowFailed:
		attr = &SubWorkflowFailedAttributes{}

	case EventType_SignalWorkflow:
		attr = &SignalWorkflowAttributes{}

	default:
		return nil, errors.New("unknown event type when deserializing attributes")
	}

	err = json.Unmarshal([]byte(attributes), &attr)
	return attr, err
}
