package redis

import (
	"fmt"

	"github.com/cschleiden/go-workflows/core"
)

type keys struct {
	// Ensure prefix ends with `:`
	prefix string
}

func newKeys(prefix string) *keys {
	if prefix != "" && prefix[len(prefix)-1] != ':' {
		prefix += ":"
	}

	return &keys{prefix: prefix}
}

// activeInstanceExecutionKey returns the key for the latest execution of the given instance
func (k *keys) activeInstanceExecutionKey(instanceID string) string {
	return fmt.Sprintf("%sactive-instance-execution:%v", k.prefix, instanceID)
}

func instanceSegment(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%v:%v", instance.InstanceID, instance.ExecutionID)
}

func (k *keys) instanceKey(instance *core.WorkflowInstance) string {
	return k.instanceKeyFromSegment(instanceSegment(instance))
}

func (k *keys) instanceKeyFromSegment(segment string) string {
	return fmt.Sprintf("%sinstance:%v", k.prefix, segment)
}

// instancesByCreation returns the key for the ZSET that contains all instances sorted by creation date. The score is the
// creation time as a unix timestamp. Used for listing all workflow instances in the diagnostics UI.
func (k *keys) instancesByCreation() string {
	return fmt.Sprintf("%sinstances-by-creation", k.prefix)
}

// instancesActive returns the key for the SET that contains all active instances. Used for reporting active workflow
// instances in stats.
func (k *keys) instancesActive() string {
	return fmt.Sprintf("%sinstances-active", k.prefix)
}

func (k *keys) instancesExpiring() string {
	return fmt.Sprintf("%sinstances-expiring", k.prefix)
}

func (k *keys) pendingEventsKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%spending-events:%v", k.prefix, instanceSegment(instance))
}

func (k *keys) historyKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%shistory:%v", k.prefix, instanceSegment(instance))
}

func historyID(sequenceID int64) string {
	return fmt.Sprintf("%v-0", sequenceID)
}

func (k *keys) futureEventsKey() string {
	return fmt.Sprintf("%sfuture-events", k.prefix)
}

func (k *keys) futureEventKey(instance *core.WorkflowInstance, scheduleEventID int64) string {
	return fmt.Sprintf("%sfuture-event:%v:%v", k.prefix, instanceSegment(instance), scheduleEventID)
}

func (k *keys) payloadKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%spayload:%v", k.prefix, instanceSegment(instance))
}
