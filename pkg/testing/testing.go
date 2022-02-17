package testing

import (
	internal "github.com/cschleiden/go-workflows/internal/testing"
	"github.com/cschleiden/go-workflows/pkg/workflow"
)

type WorkflowTester = internal.WorkflowTester

func NewWorkflowTester(wf workflow.Workflow) WorkflowTester {
	return internal.NewWorkflowTester(wf)
}
