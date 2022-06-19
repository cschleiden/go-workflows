package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
)

type CommandState int

//  ┌───────┐
//  │Pending├
//  └───────┘
//      ▼
// ┌─────────┐
// │Committed│
// └─────────┘
//      ▼
//   ┌────┐
//   │Done│
//   └────┘
const (
	CommandState_Pending CommandState = iota
	CommandState_Committed
	CommandState_Done
)

type Command interface {
	ID() int64

	Commit(clock clock.Clock) *CommandResult

	// Done marks the command as done. This transitions the state to done and indicates that the result
	// of this command has been applied.
	Done()

	State() CommandState

	Type() string
}

type CommandResult struct {
	Completed      bool
	Events         []history.Event
	ActivityEvents []history.Event
	TimerEvents    []history.Event
	WorkflowEvents []history.WorkflowEvent
}

type command struct {
	state CommandState

	id int64
}

func (c *command) commit() {
	if c.state != CommandState_Pending {
		panic("command already committed")
	}

	c.state = CommandState_Committed
}

func (c *command) ID() int64 {
	return c.id
}

func (c *command) State() CommandState {
	return c.state
}

func (c *command) Done() {
	c.state = CommandState_Done
}
