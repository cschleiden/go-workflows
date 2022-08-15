package command

type CommandState int

const (
	CommandState_Pending CommandState = iota
	CommandState_Committed
	CommandState_CancelPending
	CommandState_Canceled
	CommandState_Done
)

func (cs CommandState) String() string {
	switch cs {
	case CommandState_Pending:
		return "Pending"
	case CommandState_Committed:
		return "Committed"
	case CommandState_CancelPending:
		return "CancelPending"
	case CommandState_Canceled:
		return "Canceled"
	case CommandState_Done:
		return "Done"
	default:
		panic("unknown command state")
	}
}
