package core

type Queue string

const (
	QueueDefault = Queue("")
	QueueSystem  = Queue("_system_")
)
