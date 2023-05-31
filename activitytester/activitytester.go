package activitytester

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/activity"
	"github.com/cschleiden/go-workflows/internal/core"
	dlogger "github.com/cschleiden/go-workflows/internal/logger"
	"github.com/cschleiden/go-workflows/log"
)

func WithActivityTestState(ctx context.Context, activityID, instanceID string, logger log.Logger) context.Context {
	if logger == nil {
		logger = dlogger.NewDefaultLogger()
	}

	return activity.WithActivityState(ctx, activity.NewActivityState(activityID, core.NewWorkflowInstance(instanceID, ""), logger))
}
