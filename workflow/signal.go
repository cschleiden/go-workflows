package workflow

import (
	"fmt"

	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
)

func NewSignalChannel[T any](ctx Context, name string) Channel[T] {
	wfState := workflowstate.WorkflowState(ctx)
	return workflowstate.GetSignalChannel[T](ctx, wfState, name)
}

func SignalWorkflow[T any](ctx Context, instanceID string, name string, arg T) error {
	ctx, span := workflowtracer.Tracer(ctx).Start(ctx, "SignalWorkflow")
	defer span.End()

	wfState := workflowstate.WorkflowState(ctx)
	scheduleEventID := wfState.GetNextScheduleEventID()

	// Create command to add it to the history
	argPayload, err := converter.DefaultConverter.To(arg)
	if err != nil {
		return fmt.Errorf("converting arg to payload: %w", err)
	}

	cmd := command.NewSignalWorkflowCommand(scheduleEventID, instanceID, name, argPayload)
	wfState.AddCommand(cmd)

	return nil
}
