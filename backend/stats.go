package backend

import "github.com/cschleiden/go-workflows/workflow"

type Stats struct {
	ActiveWorkflowInstances int64

	// PendingActivities are the number of activities that are currently in the queue,
	// waiting to be processed by a worker
	PendingActivityTasks map[workflow.Queue]int64

	// PendingWorkflowTasks are the number of workflow tasks that are currently in the queue,
	// waiting to be processed by a worker
	PendingWorkflowTasks map[workflow.Queue]int64
}
