package activitytester

import (
	"context"
	"log/slog"

	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/activity"
)

// WithActivityTestState returns a context with an activity state attached that can be used for unit testing
// activities.
func WithActivityTestState(ctx context.Context, activityID, instanceID string, logger *slog.Logger) context.Context {
	if logger == nil {
		logger = slog.Default()
	}

	return activity.WithActivityState(ctx, activity.NewActivityState(activityID, 0, core.NewWorkflowInstance(instanceID, ""), logger))
}
