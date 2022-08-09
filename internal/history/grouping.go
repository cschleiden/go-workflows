package history

func EventsByWorkflowInstanceID(events []WorkflowEvent) map[string][]WorkflowEvent {
	groupedEvents := make(map[string][]WorkflowEvent)

	for _, m := range events {
		instance := *m.WorkflowInstance

		if _, ok := groupedEvents[instance.InstanceID]; !ok {
			groupedEvents[instance.InstanceID] = []WorkflowEvent{}
		}

		groupedEvents[instance.InstanceID] = append(groupedEvents[instance.InstanceID], m)
	}

	return groupedEvents
}
