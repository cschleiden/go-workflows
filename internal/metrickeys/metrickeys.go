package metrickeys

const (
	Prefix = "workflows."

	// Workflows
	WorkflowInstanceCreated  = Prefix + "workflow.created"
	WorkflowInstanceFinished = Prefix + "workflow.finished"

	WorkflowTaskScheduled = Prefix + "workflow.task.scheduled"
	WorkflowTaskProcessed = Prefix + "workflow.task.processed"
	WorkflowTaskDelay     = Prefix + "workflow.task.time_in_queue"

	WorkflowInstanceCacheSize     = Prefix + "workflow.cache.size"
	WorkflowInstanceCacheEviction = Prefix + "workflow.cache.eviction"

	// Activities
	ActivityTaskScheduled = Prefix + "activity.task.scheduled"
	ActivityTaskProcessed = Prefix + "activity.task.processed"
	ActivityTaskDelay     = Prefix + "activity.task.time_in_queue"
)

// Tag names
const (
	// Backend being used
	Backend = "backend"

	// Reason for evicting an entry from the workflow instance cache
	EvictionReason = "reason"

	SubWorkflow    = "subworkflow"
	ContinuedAsNew = "continued_as_new"

	ActivityName = "activity"
	EventName   = "event"
)
