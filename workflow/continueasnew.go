package workflow

import (
	"fmt"

	"github.com/cschleiden/go-workflows/backend/metadata"
	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/continueasnew"
)

// ContinueAsNew restarts the current workflow with the given arguments
func ContinueAsNew(ctx Context, args ...interface{}) error {
	// Capture context
	propagators := propagators(ctx)
	metadata := &metadata.WorkflowMetadata{}
	if err := injectFromWorkflow(ctx, metadata, propagators); err != nil {
		return fmt.Errorf("injecting workflow context: %w", err)
	}

	cv := contextvalue.Converter(ctx)
	inputs, err := a.ArgsToInputs(cv, args...)
	if err != nil {
		return fmt.Errorf("converting inputs for continuing workflow execution: %w", err)
	}

	return continueasnew.NewError(metadata, inputs)
}
