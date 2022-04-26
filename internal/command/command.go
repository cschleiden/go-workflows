package command

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/google/uuid"
)

type CommandType int

const (
	_ CommandType = iota

	CommandType_ScheduleActivityTask

	CommandType_ScheduleSubWorkflow
	CommandType_CancelSubWorkflow

	CommandType_ScheduleTimer
	CommandType_CancelTimer

	CommandType_SideEffect

	CommandType_CompleteWorkflow
)

type CommandState int

const (
	CommandState_Pending CommandState = iota
	CommandState_Committed
	CommandState_Done
)

type Command struct {
	State CommandState

	ID int64

	Type CommandType

	Attr interface{}
}

type ScheduleActivityTaskCommandAttr struct {
	Name   string
	Inputs []payload.Payload
}

func NewScheduleActivityTaskCommand(id int64, name string, inputs []payload.Payload) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleActivityTask,
		Attr: &ScheduleActivityTaskCommandAttr{
			Name:   name,
			Inputs: inputs,
		},
	}
}

type ScheduleSubWorkflowCommandAttr struct {
	Instance *core.WorkflowInstance
	Name     string
	Inputs   []payload.Payload
}

func NewScheduleSubWorkflowCommand(id int64, parentInstance *core.WorkflowInstance, subWorkflowInstanceID, name string, inputs []payload.Payload) Command {
	if subWorkflowInstanceID == "" {
		subWorkflowInstanceID = uuid.New().String()
	}

	return Command{
		ID:   id,
		Type: CommandType_ScheduleSubWorkflow,
		Attr: &ScheduleSubWorkflowCommandAttr{
			Instance: core.NewSubWorkflowInstance(subWorkflowInstanceID, uuid.NewString(), parentInstance.InstanceID, id),
			Name:     name,
			Inputs:   inputs,
		},
	}
}

type CancelSubWorkflowCommandAttr struct {
	SubWorkflowInstance *core.WorkflowInstance
}

func NewCancelSubWorkflowCommand(id int64, subWorkflowInstance *core.WorkflowInstance) Command {
	return Command{
		ID:   id,
		Type: CommandType_CancelSubWorkflow,
		Attr: &CancelSubWorkflowCommandAttr{
			SubWorkflowInstance: subWorkflowInstance,
		},
	}
}

type ScheduleTimerCommandAttr struct {
	At time.Time
}

func NewScheduleTimerCommand(id int64, at time.Time) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleTimer,
		Attr: &ScheduleTimerCommandAttr{
			At: at,
		},
	}
}

type CancelTimerCommandAttr struct {
	TimerScheduleEventID int64
}

func NewCancelTimerCommand(id int64, timerID int64) Command {
	return Command{
		ID:   id,
		Type: CommandType_CancelTimer,
		Attr: &CancelTimerCommandAttr{
			TimerScheduleEventID: timerID,
		},
	}
}

type SideEffectCommandAttr struct {
	Result payload.Payload
}

func NewSideEffectCommand(id int64, result payload.Payload) Command {
	return Command{
		ID:   id,
		Type: CommandType_SideEffect,
		Attr: &SideEffectCommandAttr{
			Result: result,
		},
	}
}

type CompleteWorkflowCommandAttr struct {
	Result payload.Payload
	Error  string
}

func NewCompleteWorkflowCommand(id int64, result payload.Payload, err error) Command {
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
