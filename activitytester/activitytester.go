package activitytester

import (
	"context"
	"log/slog"

	"github.com/cschleiden/go-workflows/internal/activity"
	"github.com/cschleiden/go-workflows/internal/core"
)

func WithActivityTestState(ctx context.Context, activityID, instanceID string, logger *slog.Logger) context.Context {
	if logger == nil {
		logger = slog.Default()
	}

	return activity.WithActivityState(ctx, activity.NewActivityState(activityID, core.NewWorkflowInstance(instanceID, ""), logger))
}
