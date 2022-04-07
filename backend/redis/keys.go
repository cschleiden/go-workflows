package redis

import (
	"fmt"
)

func instanceKey(instanceID string) string {
	return fmt.Sprintf("instance:%v", instanceID)
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
