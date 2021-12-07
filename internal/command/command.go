package command

type CommandType int

const (
	CommandType_None CommandType = iota
	CommandType_ScheduleActivityTask
)

type Command struct {
	ID int

	Type CommandType

	Attr interface{}
}

type ScheduleActivityTaskAttr struct {
	Name    string
	Version string
	Input   string // TODO
}

func NewScheduleActivityTaskCommand(id int, name, version, input string) Command {
	return Command{
		ID:   id,
		Type: CommandType_ScheduleActivityTask,
		Attr: ScheduleActivityTaskAttr{
			Name:    name,
			Version: version,
			Input:   input,
		},
	}
}
