package worker

type Options struct {
	// MaxParallelWorkflowTasks
	MaxParallelWorkflowTasks int

	// MaxParallelActivityTasks
	MaxParallelActivityTasks int
}

var DefaultOptions = Options{
	MaxParallelWorkflowTasks: 0,
	MaxParallelActivityTasks: 0,
}
