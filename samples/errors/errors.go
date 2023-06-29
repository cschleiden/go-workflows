package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/log"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("errors")

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func startWorkflow(ctx context.Context, c client.Client) {
	_, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world"+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(GenericErrorActivity)
	w.RegisterActivity(PanicActivity)
	w.RegisterActivity(CustomErrorActivity)
	w.RegisterActivity(WrappedErrorActivity)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) error {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "msg", msg)
	defer logger.Debug("Leaving Workflow1")

	actOptions := workflow.DefaultActivityOptions
	actOptions.RetryOptions = workflow.RetryOptions{
		MaxAttempts: 1,
	}

	_, err := workflow.ExecuteActivity[int](ctx, actOptions, GenericErrorActivity).Get(ctx)
	if err != nil {
		handleActivityError(ctx, "GenericError", logger, err)
	}

	_, err = workflow.ExecuteActivity[int](ctx, actOptions, CustomErrorActivity).Get(ctx)
	if err != nil {
		handleActivityError(ctx, "CustomError", logger, err)
	}

	_, err = workflow.ExecuteActivity[int](ctx, actOptions, PanicActivity).Get(ctx)
	if err != nil {
		handleActivityError(ctx, "Panic", logger, err)
	}

	_, err = workflow.ExecuteActivity[int](ctx, actOptions, WrappedErrorActivity).Get(ctx)
	if err != nil {
		handleActivityError(ctx, "Wrapped", logger, err)
	}

	return nil
}

func handleActivityError(ctx workflow.Context, name string, logger log.Logger, err error) {
	logger = logger.With("activity", name)

	var werr *workflow.Error
	if errors.As(err, &werr) {
		switch werr.Type {
		case "CustomError":
			logger.Error("Custom error", "err", werr)
			return
		}

		return
	}

	var perr *workflow.PanicError
	if errors.As(err, &perr) {
		logger.Error("Panic", "err", perr)
		return
	}

	logger.Error("Generic error", "err", err)
}

func GenericErrorActivity(ctx context.Context) (int, error) {
	return 0, errors.New("some activity error")
}

type CustomError struct {
	msg string
}

func (e *CustomError) Error() string {
	return e.msg
}

// Ensure CustomError implements the error interface
var _ error = (*CustomError)(nil)

func CustomErrorActivity(ctx context.Context) (int, error) {
	return 0, &CustomError{msg: "some custom error"}
}

func PanicActivity(ctx context.Context) (int, error) {
	return someFunc(), nil
}

func someFunc() int {
	panic("panic!")
}

func WrappedErrorActivity(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("wrapped error: %w", errors.New("inner error"))
}
