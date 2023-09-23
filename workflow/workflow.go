package workflow

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/core"
)

type (
	// Instance represents a workflow instance.
	Instance = core.WorkflowInstance

	// Metadata represents the metadata of a workflow instance.
	Metadata = metadata.WorkflowMetadata

	Workflow = interface{}

	Activity = interface{}
)
