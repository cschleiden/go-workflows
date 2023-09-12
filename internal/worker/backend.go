package worker

import "github.com/cschleiden/go-workflows/backend"

type BlockingBackend interface {
	backend.Backend

	// BlockOnGetTask signals that the backend implementation will block on calls
	// to GetWorkflowTask and GetActivityTask until there is a task to return,
	// meaning that these methods can be called continuously.
	BlockOnGetTask()
}
