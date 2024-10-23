package workflowstate

import (
	"context"
	"log/slog"
)

type replayHandler struct {
	replayer Replayer
	handler  slog.Handler
}

// Enabled implements slog.Handler.
func (rh *replayHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return rh.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (rh *replayHandler) Handle(ctx context.Context, r slog.Record) error {
	if rh.replayer.Replaying() {
		return nil
	}

	return rh.handler.Handle(ctx, r)
}

func (rh *replayHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &replayHandler{
		replayer: rh.replayer,
		handler:  rh.handler.WithAttrs(attrs),
	}
}

// WithGroup implements slog.Handler.
func (rh *replayHandler) WithGroup(name string) slog.Handler {
	return &replayHandler{
		replayer: rh.replayer,
		handler:  rh.handler.WithGroup(name),
	}
}

var _ slog.Handler = (*replayHandler)(nil)

type Replayer interface {
	Replaying() bool
}

func NewReplayLogger(replayer Replayer, logger *slog.Logger) *slog.Logger {
	h := logger.Handler()

	return slog.New(&replayHandler{replayer, h})
}
