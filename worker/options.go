package worker

import (
	"time"

	"github.com/cschleiden/go-workflows/workflow/executor"
)

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

	// WorkflowHeartbeatInterval is the interval between heartbeat attempts on workflow tasks. Defaults
	// to 25 seconds
	WorkflowHeartbeatInterval time.Duration

	// WorkflowPollingInterval is the interval between polling for new workflow tasks.
	// Note that if you use a backend that can wait for tasks to be available (e.g. redis) this field has no effect.
	// Defaults to 200ms.
	WorkflowPollingInterval time.Duration

	// ActivityPollingInterval is the interval between polling for new activity tasks.
	// Note that if you use a backend that can wait for tasks to be available (e.g. redis) this field has no effect.
	// Defaults to 200ms.
	ActivityPollingInterval time.Duration

	// WorkflowExecutorCache is the max size of the workflow executor cache. Defaults to 128
	WorkflowExecutorCacheSize int

	// WorkflowExecutorCache is the max TTL of the workflow executor cache. Defaults to 10 seconds
	WorkflowExecutorCacheTTL time.Duration

	// WorkflowExecutorCache is the cache to use for workflow executors. If nil, a default cache implementation
	// will be used.
	WorkflowExecutorCache executor.Cache

	WorkflowNamespaces []string

	ActivityNamespaces []string
}

var DefaultOptions = Options{
	WorkflowPollers:           2,
	WorkflowPollingInterval:   200 * time.Millisecond,
	MaxParallelWorkflowTasks:  0,
	WorkflowHeartbeatInterval: 25 * time.Second,

	WorkflowExecutorCacheSize: 128,
	WorkflowExecutorCacheTTL:  time.Second * 10,
	WorkflowExecutorCache:     nil,

	ActivityPollers:           2,
	ActivityPollingInterval:   200 * time.Millisecond,
	MaxParallelActivityTasks:  0,
	ActivityHeartbeatInterval: 25 * time.Second,
}
