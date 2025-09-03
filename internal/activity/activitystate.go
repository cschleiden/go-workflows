package activity

import (
	"context"
	"log/slog"

	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/workflow"
)

type ActivityState struct {
	ActivityID string
	Attempt    int
	Instance   *workflow.Instance
	Logger     *slog.Logger
}

func NewActivityState(activityID string, attempt int, instance *workflow.Instance, logger *slog.Logger) *ActivityState {
	return &ActivityState{
		activityID,
		attempt,
		instance,
		logger.With(
			log.ActivityIDKey, activityID,
			log.InstanceIDKey, instance.InstanceID,
			log.ExecutionIDKey, instance.ExecutionID,
			log.AttemptKey, attempt,
		),
	}
}

type key int

var activityCtxKey key

func WithActivityState(ctx context.Context, as *ActivityState) context.Context {
	return context.WithValue(ctx, activityCtxKey, as)
}

func GetActivityState(context context.Context) *ActivityState {
	return context.Value(activityCtxKey).(*ActivityState)
}
