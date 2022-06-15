package history

import "github.com/cschleiden/go-workflows/internal/core"

func EventsByWorkflowInstance(events []WorkflowEvent) map[*core.WorkflowInstance][]Event {
	groupedEvents := make(map[*core.WorkflowInstance][]Event)

	for _, m := range events {
		if _, ok := groupedEvents[m.WorkflowInstance]; !ok {
			groupedEvents[m.WorkflowInstance] = []Event{}
		}

		groupedEvents[m.WorkflowInstance] = append(groupedEvents[m.WorkflowInstance], m.HistoryEvent)
	}

	return groupedEvents
}
