package workflow

import (
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/internal/core"
)

type (
	Instance = core.WorkflowInstance
	Metadata = metadata.WorkflowMetadata
	Workflow = interface{}
)
