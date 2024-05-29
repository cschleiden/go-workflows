package backend

import "github.com/cschleiden/go-workflows/workflow"

type QueueStats struct {
	// PendingWorkflowTasks are the number of workflow tasks that are currently in the queue,
	// waiting to be processed by a worker
	PendingWorkflowTasks int64

	// PendingActivities are the number of activities that are currently in the queue,
	// waiting to be processed by a worker
	PendingActivities int64
}

type Stats struct {
	ActiveWorkflowInstances int64

	PendingTasksInQueue map[workflow.Queue]*QueueStats
}
