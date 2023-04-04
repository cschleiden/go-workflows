package command

type CancelableCommand interface {
	Command

	// Cancel cancels the command.
	Cancel()

	// HandleCancel handles a cancel event during replay
	HandleCancel()
}

type cancelableCommand struct {
	command
}

func (c *cancelableCommand) Cancel() {
	switch c.state {
	case CommandState_Pending, CommandState_Canceled:
		c.state = CommandState_Canceled
	case CommandState_Committed:
		c.state = CommandState_CancelPending
	default:
		c.invalidStateTransition(CommandState_Canceled)
	}
}

func (c *cancelableCommand) HandleCancel() {
	switch c.state {
	case CommandState_CancelPending:
		c.state = CommandState_Canceled
	default:
		c.invalidStateTransition(CommandState_Canceled)
	}
}

func (c *cancelableCommand) Done() {
	switch c.state {
	case CommandState_Committed, CommandState_Canceled:
		c.state = CommandState_Done
		if c.whenDone != nil {
			c.whenDone()
		}

	default:
		c.invalidStateTransition(CommandState_Done)
	}
}
