package worker

import "time"

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

	// ActivityHeartbeatInterval is the interval between heartbeat attempts for activity tasks. Defaults
	// to 25 seconds
	ActivityHeartbeatInterval time.Duration

	// HeartbeatWorkflowTasks determines if the lock on workflow tasks should be periodically
	// extended while they are being processed. Given that workflow executions should be
	// very quick, this is usually not necessary.
	HeartbeatWorkflowTasks bool

	// WorkflowHeartbeatInterval is the interval between heartbeat attempts on workflow tasks, when enabled.
	WorkflowHeartbeatInterval time.Duration
}

var DefaultOptions = Options{
	WorkflowPollers:           2,
	ActivityPollers:           2,
	MaxParallelWorkflowTasks:  0,
	MaxParallelActivityTasks:  0,
	ActivityHeartbeatInterval: 25 * time.Second,
	WorkflowHeartbeatInterval: 25 * time.Second,
}
