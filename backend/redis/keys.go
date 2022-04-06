package redis

import (
	"fmt"
)

func instanceKey(instanceID string) string {
	return fmt.Sprintf("instance:%v", instanceID)
}

func pendingEventsKey(instanceID string) string {
	return fmt.Sprintf("pending-events:%v", instanceID)
}

func historyKey(instanceID string) string {
	return fmt.Sprintf("history:%v", instanceID)
}

func workflowsKey() string {
	return "workflows"
}

func workflowsProcessingKey() string {
	return "workflows-processing"
}

func activitiesKey() string {
	return "activities"
}

func activitiesProcessingKey() string {
	return "activities-processing"
}

func activityKey(activityID string) string {
	return fmt.Sprintf("activity-%v", activityID)
}
