package testing

import (
	internal "github.com/cschleiden/go-dt/internal/testing"
	"github.com/cschleiden/go-dt/pkg/workflow"
)

type WorkflowTester = internal.WorkflowTester

func NewWorkflowTester(wf workflow.Workflow) WorkflowTester {
	return internal.NewWorkflowTester(wf)
}
