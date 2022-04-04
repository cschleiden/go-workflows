package redis

import (
	"fmt"

	"github.com/cschleiden/go-workflows/internal/core"
)

func instanceKey(instance core.WorkflowInstance) string {
	return fmt.Sprintf("instance-%v", instance.GetInstanceID())
}

func pendingEventsKey(instance core.WorkflowInstance) string {
	return fmt.Sprintf("pending-%v", instance.GetInstanceID())
}

func pendingInstancesKey() string {
	return "pending-instances"
}

func lockedInstancesKey() string {
	return "locked-instances"
}
