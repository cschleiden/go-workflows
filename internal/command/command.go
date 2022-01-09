package command

import (
	"time"

	"github.com/cschleiden/go-dt/internal/payload"
)

type CommandType int

const (
	_ CommandType = iota

	CommandType_ScheduleActivityTask

	CommandType_ScheduleSubWorkflow

	CommandType_ScheduleTimer
	CommandType_CancelTimer

	CommandType_CompleteWorkflow
)

type Command struct {
	ID int

	Type CommandType

	Attr interface{}
}

type ScheduleActivityTaskCommandAttr struct {
	Name    string
	Version string
	Inputs  []payload.Payload
}

func NewScheduleActivityTaskCommand(id int, name, version string, inputs []payload.Payload) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleActivityTask,
		Attr: &ScheduleActivityTaskCommandAttr{
			Name:    name,
			Version: version,
			Inputs:  inputs,
		},
	}
}

type ScheduleSubWorkflowCommandAttr struct {
	Name    string
	Version string
	Inputs  []payload.Payload
}

func NewScheduleSubWorkflowCommand(id int, name, version string, inputs []payload.Payload) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleSubWorkflow,
		Attr: &ScheduleSubWorkflowCommandAttr{
			Name:    name,
			Version: version,
			Inputs:  inputs,
		},
	}
}

type ScheduleTimerCommandAttr struct {
	At time.Time
}

func NewScheduleTimerCommand(id int, at time.Time) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleTimer,
		Attr: &ScheduleTimerCommandAttr{
			At: at,
		},
	}
}

type CancelTimerCommandAttr struct {
	TimerID int
}

func NewCancelTimerCommand(id, timerID int) Command {
	return Command{
		ID:   id,
		Type: CommandType_CancelTimer,
		Attr: &CancelTimerCommandAttr{
			TimerID: timerID,
		},
	}
}

type CompleteWorkflowCommandAttr struct {
	Result payload.Payload
	Error  string
}

func NewCompleteWorkflowCommand(id int, result payload.Payload, err error) Command {
	var error string
	if err != nil {
		error = err.Error()
	}

	return Command{
		ID:   id,
		Type: CommandType_CompleteWorkflow,
		Attr: &CompleteWorkflowCommandAttr{
			Result: result,
			Error:  error,
		},
	}
}
