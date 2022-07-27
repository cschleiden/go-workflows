package history

import "github.com/cschleiden/go-workflows/internal/core"

func EventsByWorkflowInstance(events []WorkflowEvent) map[core.WorkflowInstance][]Event {
	groupedEvents := make(map[core.WorkflowInstance][]Event)

	for _, m := range events {
		instance := *m.WorkflowInstance

		if _, ok := groupedEvents[instance]; !ok {
			groupedEvents[instance] = []Event{}
		}

		groupedEvents[instance] = append(groupedEvents[instance], m.HistoryEvent)
	}

	return groupedEvents
}
