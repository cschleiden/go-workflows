package command

import (
	"fmt"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
)

type Command interface {
	ID() int64

	Type() string

	// State returns the current state of the command.
	State() CommandState

	// Execute processes the command in its current state and moves it to the next state.
	Execute(clock.Clock) *CommandResult

	// Commit marks the command as committed without executing it.
	Commit()

	// Done marks the command as done. This transitions the state to done and indicates that the result
	// of this command has been applied.
	Done()
}

type CommandResult struct {
	Completed      bool
	Events         []*history.Event
	ActivityEvents []*history.Event
	TimerEvents    []*history.Event
	WorkflowEvents []history.WorkflowEvent
}

type command struct {
	name string

	state CommandState

	id int64
}

func (c *command) ID() int64 {
	return c.id
}

func (c *command) State() CommandState {
	return c.state
}

func (c *command) Type() string {
	return c.name
}

func (c *command) Commit() {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Committed
	default:
		c.invalidStateTransition(CommandState_Committed)
	}
}

func (c *command) Done() {
	switch c.state {
	case CommandState_Committed:
		c.state = CommandState_Done
	default:
		c.invalidStateTransition(CommandState_Done)
	}
}

func (c *command) invalidStateTransition(state CommandState) {
	panic(fmt.Errorf("invalid state transition for command %s: %s -> %s", c.name, c.State().String(), state.String()))
}
