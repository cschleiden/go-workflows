package redis

import (
	"fmt"
)

func instanceKey(instanceID string) string {
	return fmt.Sprintf("instance:%v", instanceID)
}

// instancesByCreation returns the key for the ZSET that contains all instances sorted by creation date. The score is the
// creation time. Used for listing all workflow instances in the diagnostics UI.
func instancesByCreation() string {
	return "instances-by-creation"
}

func pendingEventsKey(instanceID string) string {
	return fmt.Sprintf("pending-events:%v", instanceID)
}

func historyKey(instanceID string) string {
	return fmt.Sprintf("history:%v", instanceID)
}

func historyID(sequenceID int64) string {
	return fmt.Sprintf("%v-0", sequenceID)
}

func futureEventsKey() string {
	return "future-events"
}

func futureEventKey(instanceID string, scheduleEventID int64) string {
	return fmt.Sprintf("future-event:%v:%v", instanceID, scheduleEventID)
}
