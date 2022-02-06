package worker

type Options struct {
	// MaxParallelWorkflowTasks
	MaxParallelWorkflowTasks int

	// MaxParallelActivityTasks
	MaxParallelActivityTasks int

	HeartbeatWorkflowTasks bool
}

var DefaultOptions = Options{
	MaxParallelWorkflowTasks: 0,
	MaxParallelActivityTasks: 0,
}
