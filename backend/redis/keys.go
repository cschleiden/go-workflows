package redis

import (
	"fmt"

	"github.com/cschleiden/go-workflows/internal/core"
)

// activeInstanceExecutionKey returns the key for the latest execution of the given instance
func activeInstanceExecutionKey(keyPrefix string, instanceID string) string {
	return fmt.Sprintf("%vactive-instance-execution:%v", keyPrefix, instanceID)
}

func instanceSegment(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%v:%v", instance.InstanceID, instance.ExecutionID)
}

func instanceKey(keyPrefix string, instance *core.WorkflowInstance) string {
	return instanceKeyFromSegment(keyPrefix, instanceSegment(instance))
}

func instanceKeyFromSegment(keyPrefix string, segment string) string {
	return fmt.Sprintf("%vinstance:%v", keyPrefix, segment)
}

// instancesByCreation returns the key for the ZSET that contains all instances sorted by creation date. The score is the
// creation time. Used for listing all workflow instances in the diagnostics UI.
func instancesByCreation(keyPrefix string) string {
	return keyPrefix + "instances-by-creation"
}

func instancesActive(keyPrefix string) string {
	return keyPrefix + "instances-active"
}

func instancesExpiring(keyPrefix string) string {
	return keyPrefix + "instances-expiring"
}

func pendingEventsKey(keyPrefix string, instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%vpending-events:%v", keyPrefix, instanceSegment(instance))
}

func historyKey(keyPrefix string, instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%vhistory:%v", keyPrefix, instanceSegment(instance))
}

func historyID(sequenceID int64) string {
	return fmt.Sprintf("%v-0", sequenceID)
}

func futureEventsKey(keyPrefix string) string {
	return keyPrefix + "future-events"
}

func futureEventKey(instance *core.WorkflowInstance, scheduleEventID int64) string {
	return fmt.Sprintf("future-event:%v:%v:%v", instance.InstanceID, instance.ExecutionID, scheduleEventID)
}
