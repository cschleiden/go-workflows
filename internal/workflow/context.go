package workflow

import (
	"github.com/cschleiden/go-dt/internal/command"
)

type Context interface {
	// Are we currently replaying?
	Replaying() bool

	// ExecuteActivity schedules the given activity to be executed
	ExecuteActivity(name string) (Future, error) // TODO: inputs
}

func newWorkflowContext() *contextImpl {
	return &contextImpl{
		commands:    []command.Command{},
		id:          0,
		openFutures: map[int]Future{},
	}
}

type contextImpl struct {
	commands []command.Command

	id          int
	openFutures map[int]Future

	cs *coState
}

func (c *contextImpl) Replaying() bool {
	return false // TODO
}

func (c *contextImpl) ExecuteActivity(name string) (Future, error) {
	id := c.id
	c.id++

	command := command.NewScheduleActivityTaskCommand(id, name, "", "")
	c.commands = append(c.commands, command)

	f := newFuture(c.cs)
	c.openFutures[id] = f

	return f, nil
}
