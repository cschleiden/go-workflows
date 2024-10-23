package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/payload"
)

type StartTraceCommand struct {
	command

	spanID payload.Payload
}

var _ Command = (*StartTraceCommand)(nil)

// Command transitions are
// Pending -> Done : Trace has been started, Span has been created

func NewStartTraceCommand(id int64) *StartTraceCommand {
	return &StartTraceCommand{
		command: command{
			id:    id,
			name:  "StartTrace",
			state: CommandState_Pending,
		},
	}
}

func (c *StartTraceCommand) SetSpanID(spanID payload.Payload) {
	c.spanID = spanID
}

func (c *StartTraceCommand) Commit() {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Done

	default:
		c.invalidStateTransition(CommandState_Done)
	}
}

func (c *StartTraceCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Done

		return &CommandResult{
			Events: []*history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_TraceStarted,
					&history.TraceStartedAttributes{
						SpanID: c.spanID,
					},
					history.ScheduleEventID(c.id),
				),
			},
		}
	}

	return nil
}

func (c *StartTraceCommand) Done() {
	switch c.state {
	case CommandState_Pending, CommandState_Committed:
		c.state = CommandState_Done
		if c.whenDone != nil {
			c.whenDone()
		}

	default:
		c.invalidStateTransition(CommandState_Done)
	}
}
