package workflow

import (
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/cschleiden/go-dt/pkg/converter"
	"github.com/pkg/errors"
)

type Context interface {
	// Are we currently replaying?
	Replaying() bool

	// ExecuteActivity schedules the given activity to be executed
	ExecuteActivity(name string, args ...interface{}) (sync.Future, error)
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

func (c *contextImpl) ExecuteActivity(name string, args ...interface{}) (sync.Future, error) {
	eventID := c.eventID
	c.eventID++

	inputs := make([][]byte, 0)
	for _, arg := range args {
		input, err := converter.DefaultConverter.To(arg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert activity input")
		}
		inputs = append(inputs, input)
	}

	// TOOO: Validate arguments against activity registration}

	command := command.NewScheduleActivityTaskCommand(eventID, name, "", inputs)
	c.commands = append(c.commands, command)

	f := sync.NewFuture(c.cr)
	c.pendingFutures[eventID] = f

	return f, nil
}

func (c *contextImpl) AddCommand(cmd command.Command) {
	c.commands = append(c.commands, cmd)
}
