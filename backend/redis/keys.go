package redis

import (
	"fmt"
)

func instanceKey(instanceID string) string {
	return fmt.Sprintf("instance:%v", instanceID)
}

func instancesByCreation() string {
	return "instances-by-creation"
}

func subInstanceKey(instanceID string) string {
	return fmt.Sprintf("sub-instance:%v", instanceID)
}

func pendingEventsKey(instanceID string) string {
	return fmt.Sprintf("pending-events:%v", instanceID)
}

func historyKey(instanceID string) string {
	return fmt.Sprintf("history:%v", instanceID)
}

func futureEventsKey() string {
	return "future-events"
}

func futureEventKey(instanceID string, scheduleEventID int64) string {
	return fmt.Sprintf("future-event:%v:%v", instanceID, scheduleEventID)
}
