package workflow

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/core"
)

type (
	Instance = core.WorkflowInstance
	Metadata = metadata.WorkflowMetadata
	Activity = interface{}
	Workflow = interface{}
)
