package workflow

import (
	"fmt"

	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/continueasnew"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
)

// ContinueAsNew restarts the current workflow with the given arguments
func ContinueAsNew(ctx Context, args ...interface{}) error {
	// Capture context
	propagators := contextpropagation.Propagators(ctx)
	metadata := &core.WorkflowMetadata{}
	if err := contextpropagation.InjectFromWorkflow(ctx, metadata, propagators); err != nil {
		return fmt.Errorf("injecting workflow context: %w", err)
	}

	cv := converter.GetConverter(ctx)
	inputs, err := a.ArgsToInputs(cv, args...)
	if err != nil {
		return fmt.Errorf("converting inputs for continuing workflow execution: %w", err)
	}

	return continueasnew.NewError(metadata, inputs)
}
