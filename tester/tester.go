package testing

import (
	internal "github.com/cschleiden/go-workflows/internal/tester"
	"github.com/cschleiden/go-workflows/workflow"
)

type WorkflowTester = internal.WorkflowTester

func NewWorkflowTester(wf workflow.Workflow) WorkflowTester {
	return internal.NewWorkflowTester(wf)
}
