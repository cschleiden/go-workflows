package worker

type Options struct {
	// WorkflowsPollers is the number of pollers to start. Defaults to 2.
	WorkflowPollers int

	// MaxParallelWorkflowTasks determines the maximum number of concurrent workflow tasks processed
	// by the worker. The default is 0 which is no limit.
	MaxParallelWorkflowTasks int

	// ActivityPollers is the number of pollers to start. Defaults to 2.
	ActivityPollers int

	// MaxParallelActivityTasks determines the maximum number of concurrent activity tasks processed
	// by the worker. The default is 0 which is no limit.
	MaxParallelActivityTasks int

	// HeartbeatWorkflowTasks determines if the lock on workflow tasks should be periodically
	// extended while they are being processed. Given that workflow executions should be
	// very quick, this is usually not necessary.
	HeartbeatWorkflowTasks bool
}

var DefaultOptions = Options{
	WorkflowPollers:          2,
	ActivityPollers:          2,
	MaxParallelWorkflowTasks: 0,
	MaxParallelActivityTasks: 0,
}
