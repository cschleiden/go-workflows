package core

import "github.com/google/uuid"

type WorkflowInstance struct {
	InstanceID uuid.UUID

	ExecutionID uuid.UUID
}
