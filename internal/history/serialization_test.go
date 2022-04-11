package history

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRoundtripJSON(t *testing.T) {
	event := NewHistoryEvent(42, time.Now(), EventType_WorkflowExecutionStarted, &ExecutionStartedAttributes{
		Name: "my-workflow",
	})

	b, err := json.Marshal(event)
	require.NoError(t, err)

	var event2 Event
	uerr := json.Unmarshal(b, &event2)
	require.NoError(t, uerr)

	require.Equal(t, event.ID, event2.ID)
	require.Equal(t, event.SequenceID, event2.SequenceID)
	require.Equal(t, event.Type, event2.Type)
	require.Equal(t, event.VisibleAt, event2.VisibleAt)
	require.Equal(t, event.Attributes, event2.Attributes)
}
