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

	r := EventsByWorkflowInstanceID([]WorkflowEvent{
		{
			WorkflowInstance: core.NewWorkflowInstance(id, "exid"),
			HistoryEvent:     NewPendingEvent(time.Now(), EventType_SubWorkflowScheduled, &SubWorkflowScheduledAttributes{}),
		},
		{
			WorkflowInstance: core.NewWorkflowInstance(id, ""),
			HistoryEvent:     NewPendingEvent(time.Now(), EventType_SignalReceived, &SubWorkflowScheduledAttributes{}),
		},
	})

	require.Len(t, r, 1)
	require.Len(t, r[id], 2)
	require.Equal(t, r[id][0].HistoryEvent.Type, EventType_SubWorkflowScheduled)
	require.Equal(t, r[id][1].HistoryEvent.Type, EventType_SignalReceived)
}
