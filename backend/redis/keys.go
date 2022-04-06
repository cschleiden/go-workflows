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
		queue:      queueKey(t),
		processing: processingKey(t),
		lease:      leaseKey(t),
	}
}

func queueKey(t string) string {
	return "queue:" + t
}

func processingKey(t string) string {
	return "processing:" + t
}

func leaseKey(t string) string {
	return "lease:" + t
}
