package simple_split_worker

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/workflow"

	errs "github.com/pkg/errors"
)

func Workflow1(ctx workflow.Context, msg string) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "msg", msg)
	defer logger.Debug("Leaving Workflow1")

	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		return "", errs.Wrap(err, "error getting activity 1 result")
	}
	// logger.Debug("R1 result", "r1", r1)

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		return "", errs.Wrap(err, "error getting activity 2 result")
	}
	// logger.Debug("R2 result", "r2", r2)

	return fmt.Sprintf("%s-%d,%d", msg, r1, r2), nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	return 12, nil
}
