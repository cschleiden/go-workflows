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
		commands:    []command.Command{},
		id:          0,
		openFutures: map[int]sync.Future{},
		cr:          cr,
	}
}

type contextImpl struct {
	commands []command.Command

	id          int
	openFutures map[int]sync.Future

	cr sync.Coroutine
}

func (c *contextImpl) Replaying() bool {
	return false // TODO
}

func (c *contextImpl) ExecuteActivity(name string) (sync.Future, error) {
	id := c.id
	c.id++

	command := command.NewScheduleActivityTaskCommand(id, name, "", "")
	c.commands = append(c.commands, command)

	f := sync.NewFuture(c.cr)
	c.openFutures[id] = f

	return f, nil
}
