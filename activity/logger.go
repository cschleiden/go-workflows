package activity

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/activity"
	"github.com/cschleiden/go-workflows/log"
)

// Logger returns a logger with the workflow instance this activity is executed for set as default fields
func Logger(ctx context.Context) log.Logger {
	return activity.GetActivityState(ctx).Logger
}
