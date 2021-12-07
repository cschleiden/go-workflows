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
	Input   string // TODO: Activity inputs
}

func NewScheduleActivityTaskCommand(id int, name, version, input string) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleActivityTask,
		Attr: ScheduleActivityTaskCommandAttr{
			Name:    name,
			Version: version,
			Input:   input,
		},
	}
}

type CompleteWorkflowCommandAttr struct {
}

func NewCompleteWorkflowCommand(id int) Command {
	return Command{
		ID:   id,
		Type: CommandType_CompleteWorkflow,
		Attr: CompleteWorkflowCommandAttr{},
	}
}
