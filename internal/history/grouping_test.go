package history

import (
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGrouping_MultipleEventsSameInstance(t *testing.T) {
	id := uuid.NewString()
	instance := core.NewWorkflowInstance(id, "exid")

	r := EventsByWorkflowInstance([]WorkflowEvent{
		{
			WorkflowInstance: instance,
			HistoryEvent:     NewPendingEvent(time.Now(), EventType_SubWorkflowScheduled, &SubWorkflowScheduledAttributes{}),
		},
		{
			WorkflowInstance: instance,
			HistoryEvent:     NewPendingEvent(time.Now(), EventType_SignalReceived, &SubWorkflowScheduledAttributes{}),
		},
	})

	require.Len(t, r, 1)
	require.Len(t, r[*instance], 2)
	require.Equal(t, r[*instance][0].HistoryEvent.Type, EventType_SubWorkflowScheduled)
	require.Equal(t, r[*instance][1].HistoryEvent.Type, EventType_SignalReceived)
}
