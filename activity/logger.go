package activity

import (
	"context"
	"log/slog"

	"github.com/cschleiden/go-workflows/internal/activity"
)

// Logger returns a logger with the workflow instance this activity is executed for set as default fields
func Logger(ctx context.Context) *slog.Logger {
	return activity.GetActivityState(ctx).Logger
}
