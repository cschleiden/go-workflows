package command

type CommandType int

const (
	CommandType_None CommandType = iota
	CommandType_ScheduleActivityTask
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
	Inputs  [][]byte
}

func NewScheduleActivityTaskCommand(id int, name, version string, inputs [][]byte) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleActivityTask,
		Attr: ScheduleActivityTaskCommandAttr{
			Name:    name,
			Version: version,
			Inputs:  inputs,
		},
	}
}

type CompleteWorkflowCommandAttr struct {
	Result []byte
}

func NewCompleteWorkflowCommand(id int, result []byte) Command {
	return Command{
		ID:   id,
		Type: CommandType_CompleteWorkflow,
		Attr: CompleteWorkflowCommandAttr{
			Result: result,
		},
	}
}
