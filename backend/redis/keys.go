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

func activityKey(activityID string) string {
	return fmt.Sprintf("activity-%v", activityID)
}

// Queue keys

type keys struct {
	queue      string
	processing string
	lease      string
}

func queueKeys(t string) *keys {
	return &keys{
		queue:      "queue:" + t,
		processing: "processing:" + t,
		lease:      "lease:" + t,
	}
}
