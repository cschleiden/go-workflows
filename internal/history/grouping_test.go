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

	r := EventsByWorkflowInstance([]WorkflowEvent{
		{
			WorkflowInstance: core.NewWorkflowInstance(id, "exid"),
			HistoryEvent:     NewPendingEvent(time.Now(), EventType_SubWorkflowScheduled, &SubWorkflowScheduledAttributes{}),
		},
		{
			WorkflowInstance: core.NewWorkflowInstance(id, "exid"),
			HistoryEvent:     NewPendingEvent(time.Now(), EventType_SubWorkflowScheduled, &SubWorkflowScheduledAttributes{}),
		},
	})

	require.Len(t, r, 1)
	require.Len(t, r[*core.NewWorkflowInstance(id, "exid")], 2)
}
