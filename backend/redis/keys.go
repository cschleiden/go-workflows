package redis

import (
	"fmt"

	"github.com/cschleiden/go-workflows/core"
)

// activeInstanceExecutionKey returns the key for the latest execution of the given instance
func activeInstanceExecutionKey(instanceID string) string {
	return fmt.Sprintf("active-instance-execution:%v", instanceID)
}

func instanceSegment(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%v:%v", instance.InstanceID, instance.ExecutionID)
}

func instanceKey(instance *core.WorkflowInstance) string {
	return instanceKeyFromSegment(instanceSegment(instance))
}

func instanceKeyFromSegment(segment string) string {
	return fmt.Sprintf("instance:%v", segment)
}

// instancesByCreation returns the key for the ZSET that contains all instances sorted by creation date. The score is the
// creation time. Used for listing all workflow instances in the diagnostics UI.
func instancesByCreation() string {
	return "instances-by-creation"
}

func instancesActive() string {
	return "instances-active"
}

func instancesExpiring() string {
	return "instances-expiring"
}

func instanceIDs() string {
	return "instances"
}

func pendingEventsKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("pending-events:%v", instanceSegment(instance))
}

func historyKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("history:%v", instanceSegment(instance))
}

func historyID(sequenceID int64) string {
	return fmt.Sprintf("%v-0", sequenceID)
}

func futureEventsKey() string {
	return "future-events"
}

func futureEventKey(instance *core.WorkflowInstance, scheduleEventID int64) string {
	return fmt.Sprintf("future-event:%v:%v", instanceSegment(instance), scheduleEventID)
}

func payloadKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("payload:%v", instanceSegment(instance))
}
