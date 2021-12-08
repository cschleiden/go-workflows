package workflow

import (
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/sync"
)

type Context interface {
	// Are we currently replaying?
	Replaying() bool

	// ExecuteActivity schedules the given activity to be executed
	ExecuteActivity(name string) (sync.Future, error) // TODO: inputs
}

func newWorkflowContext(cr sync.Coroutine) *contextImpl {
	return &contextImpl{
		commands:       []command.Command{},
		eventID:        0,
		pendingFutures: map[int]sync.Future{},
		cr:             cr,
	}
}

type contextImpl struct {
	commands []command.Command

	eventID        int
	pendingFutures map[int]sync.Future

	cr sync.Coroutine

	replaying bool
}

func (c *contextImpl) Replaying() bool {
	return c.replaying
}

func (c *contextImpl) SetReplaying(replaying bool) {
	c.replaying = replaying
}

func (c *contextImpl) ExecuteActivity(name string) (sync.Future, error) {
	eventID := c.eventID
	c.eventID++

	command := command.NewScheduleActivityTaskCommand(eventID, name, "", "")
	c.commands = append(c.commands, command)

	f := sync.NewFuture(c.cr)
	c.pendingFutures[eventID] = f

	return f, nil
}

func (c *contextImpl) AddCommand(cmd command.Command) {
	c.commands = append(c.commands, cmd)
}
