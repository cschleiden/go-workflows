package workflow

import (
	"github.com/cschleiden/go-dt/internal/commands"
)

type Context interface {
	Replaying() bool

	RegisterResult()

	ExecuteActivity(name string) (Future, error) // TODO: inputs
}

func newWorkflowContext() *contextImpl {
	return &contextImpl{
		commands:    []commands.Command{},
		id:          0,
		openFutures: map[int]Future{},
	}
}

type contextImpl struct {
	commands []commands.Command

	id          int
	openFutures map[int]Future

	cs *coState
}

func (c *contextImpl) Replaying() bool {
	return false // TODO
}

func (c *contextImpl) RegisterResult() {
	panic("not implemented")
}

func (c *contextImpl) ExecuteActivity(name string) (Future, error) {
	id := c.id
	c.id++

	command := commands.NewScheduleActivityTaskCommand(id, name, "", "")
	c.commands = append(c.commands, command)

	f := newFuture(c.cs)
	c.openFutures[id] = f

	return f, nil
}
