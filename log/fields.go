package log

const (
	NamespaceKey = "workflows"

	ActivityIDKey   = NamespaceKey + ".activity.id"
	ActivityNameKey = NamespaceKey + ".activity.name"
	InstanceIDKey   = NamespaceKey + ".instance.id"

	WorkflowNameKey = NamespaceKey + ".workflow.name"

	SignalNameKey = NamespaceKey + ".signal.name"

	SeqIDKey       = NamespaceKey + ".seq_id"
	IsReplayingKey = NamespaceKey + ".is_replaying"

	EventTypeKey       = NamespaceKey + ".event.type"
	EventIDKey         = NamespaceKey + ".event.id"
	ScheduleEventIDKey = NamespaceKey + ".event.schedule_event_id"

	TaskIDKey             = NamespaceKey + ".task.id"
	TaskLastSequenceIDKey = NamespaceKey + ".task.last_sequence_id"
	TaskSequenceIDKey     = NamespaceKey + ".task.sequence_id"
	LocalSequenceIDKey    = NamespaceKey + ".task.local_sequence_id"
	WorkflowCompletedKey  = NamespaceKey + ".task.workflow_completed"
	ExecutedEventsKey     = NamespaceKey + ".task.executed_events"
	NewEventsKey          = NamespaceKey + ".task.new_events"

	AttemptKey  = NamespaceKey + ".attempt"
	DurationKey = NamespaceKey + ".duration_ms"

	// NowKey is the time at which a timer was scheduled
	NowKey = NamespaceKey + ".timer.now"
	// At is the time at which a timer is scheduled to fire
	AtKey = NamespaceKey + ".timer.at"
)
