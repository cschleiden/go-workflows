package command

import (
	"time"

	"github.com/cschleiden/go-dt/internal/payload"
)

type CommandType int

const (
	CommandType_None CommandType = iota

	CommandType_ScheduleActivityTask

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
}

func NewCompleteWorkflowCommand(id int, result payload.Payload) Command {
	return Command{
		ID:   id,
		Type: CommandType_CompleteWorkflow,
		Attr: &CompleteWorkflowCommandAttr{
			Result: result,
		},
	}
}
