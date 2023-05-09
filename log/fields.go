package log

const (
	NamespaceKey = "workflows"

	ActivityIDKey   = NamespaceKey + ".activity.id"
	ActivityNameKey = NamespaceKey + ".activity.name"
	InstanceIDKey   = NamespaceKey + ".instance.id"
	SeqIDKey        = NamespaceKey + "." + "seq_id"
	IsReplayingKey  = NamespaceKey + "." + "is_replaying"

	EventTypeKey       = NamespaceKey + ".event.type"
	EventIDKey         = NamespaceKey + ".event.id"
	ScheduleEventIDKey = NamespaceKey + ".event.schedule_event_id"

	TaskIDKey             = NamespaceKey + ".task.id"
	TaskLastSequenceIDKey = NamespaceKey + ".task.last_sequence_id"
	TaskSequenceIDKey     = NamespaceKey + ".task.sequence_id"
	LocalSequenceIDKey    = NamespaceKey + ".task.local_sequence_id"
	WorkflowCompletedKey  = NamespaceKey + ".task.workflow_completed"
	ExecutedEventsKey     = NamespaceKey + ".task.executed_events"
)
