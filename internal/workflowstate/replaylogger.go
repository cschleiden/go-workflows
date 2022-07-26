package workflowstate

import (
	"github.com/ticctech/go-workflows/log"
)

type replayLogger struct {
	state  *WfState
	logger log.Logger
}

func NewReplayLogger(state *WfState, logger log.Logger) log.Logger {
	return &replayLogger{state, logger}
}

func (r *replayLogger) Debug(msg string, fields ...interface{}) {
	if !r.state.replaying {
		r.logger.Debug(msg, fields...)
	}
}

// Error implements log.Logger
func (r *replayLogger) Error(msg string, fields ...interface{}) {
	if !r.state.replaying {
		r.logger.Error(msg, fields...)
	}
}

// Panic implements log.Logger
func (r *replayLogger) Panic(msg string, fields ...interface{}) {
	if !r.state.replaying {
		r.logger.Panic(msg, fields...)
	}
}

// Warn implements log.Logger
func (r *replayLogger) Warn(msg string, fields ...interface{}) {
	if !r.state.replaying {
		r.logger.Warn(msg, fields...)
	}
}

// With implements log.Logger
func (r *replayLogger) With(fields ...interface{}) log.Logger {
	return NewReplayLogger(r.state, r.logger.With(fields...))
}
