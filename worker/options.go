package worker

import (
	"time"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/cschleiden/go-workflows/workflow/executor"
)

type WorkflowWorkerOptions struct {
	// WorkflowsPollers is the number of pollers to start. Defaults to 2.
	WorkflowPollers int

	// MaxParallelWorkflowTasks determines the maximum number of concurrent workflow tasks processed
	// by the worker. The default is 0 which is no limit.
	MaxParallelWorkflowTasks int

	// WorkflowHeartbeatInterval is the interval between heartbeat attempts on workflow tasks. Defaults
	// to 25 seconds
	WorkflowHeartbeatInterval time.Duration

	// WorkflowPollingInterval is the interval between polling for new workflow tasks.
	// Note that if you use a backend that can wait for tasks to be available (e.g. redis) this field has no effect.
	// Defaults to 200ms.
	WorkflowPollingInterval time.Duration

	// WorkflowExecutorCache is the max size of the workflow executor cache. Defaults to 128
	WorkflowExecutorCacheSize int

	// WorkflowExecutorCache is the max TTL of the workflow executor cache. Defaults to 10 seconds
	WorkflowExecutorCacheTTL time.Duration

	// WorkflowExecutorCache is the cache to use for workflow executors. If nil, a default cache implementation
	// will be used.
	WorkflowExecutorCache executor.Cache

	// WorkflowQueues are the queue the worker listens to
	WorkflowQueues []workflow.Queue
}

type Options struct {
	WorkflowWorkerOptions
	ActivityWorkerOptions

	// SingleWorkerMode enables automatic registration of workflows and activities
	// when they are used in workflows. This is useful for simple scenarios where
	// you don't want to explicitly register each workflow and activity.
	SingleWorkerMode bool
}

type ActivityWorkerOptions struct {
	// ActivityPollers is the number of pollers to start. Defaults to 2.
	ActivityPollers int

	// MaxParallelActivityTasks determines the maximum number of concurrent activity tasks processed
	// by the worker. The default is 0 which is no limit.
	MaxParallelActivityTasks int

	// ActivityHeartbeatInterval is the interval between heartbeat attempts for activity tasks. Defaults
	// to 25 seconds
	ActivityHeartbeatInterval time.Duration

	// ActivityPollingInterval is the interval between polling for new activity tasks.
	// Note that if you use a backend that can wait for tasks to be available (e.g. redis) this field has no effect.
	// Defaults to 200ms.
	ActivityPollingInterval time.Duration

	// ActivityQueues are the queues the worker listens to
	ActivityQueues []workflow.Queue
}

var DefaultOptions = Options{
	WorkflowWorkerOptions: WorkflowWorkerOptions{
		WorkflowPollers:           2,
		WorkflowPollingInterval:   200 * time.Millisecond,
		MaxParallelWorkflowTasks:  0,
		WorkflowHeartbeatInterval: 25 * time.Second,

		WorkflowExecutorCacheSize: 128,
		WorkflowExecutorCacheTTL:  time.Second * 10,
		WorkflowExecutorCache:     nil,
	},

	ActivityWorkerOptions: ActivityWorkerOptions{
		ActivityPollers:           2,
		ActivityPollingInterval:   200 * time.Millisecond,
		MaxParallelActivityTasks:  0,
		ActivityHeartbeatInterval: 25 * time.Second,
	},

	// By default, SingleWorkerMode is disabled
	SingleWorkerMode: false,
}
