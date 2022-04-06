package redis

import (
	"fmt"
)

func instanceKey(instanceID string) string {
	return fmt.Sprintf("instance-%v", instanceID)
}

func pendingEventsKey(instanceID string) string {
	return fmt.Sprintf("pending-events-%v", instanceID)
}

func eventsKey(instanceID string) string {
	return fmt.Sprintf("events-%v", instanceID)
}

func pendingInstancesKey() string {
	return "pending-instances"
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
