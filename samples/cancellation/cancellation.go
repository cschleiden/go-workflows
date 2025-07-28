package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	b := samples.GetBackend("cancellation")

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2

	cancel()
}

func startWorkflow(ctx context.Context, c *client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	time.Sleep(2 * time.Second)

	if err := c.CancelWorkflowInstance(ctx, wf); err != nil {
		panic("could not cancel workflow")
	}
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(Workflow2)
	w.RegisterActivity(ActivityCancel)
	w.RegisterActivity(ActivitySkip)
	w.RegisterActivity(ActivitySuccess)
	w.RegisterActivity(ActivityCleanup)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "msg", msg)
	defer logger.Debug("Leaving Workflow1")

	defer func() {
		if errors.Is(ctx.Err(), workflow.Canceled) {
			logger.Debug("Workflow1 was canceled")

			logger.Debug("Do cleanup")
			ctx := workflow.NewDisconnectedContext(ctx)
			if _, err := workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, ActivityCleanup).Get(ctx); err != nil {
				panic("could not execute cleanup activity")
			}
			logger.Debug("Done with cleanup")
		}
	}()

	logger.Debug("schedule ActivitySuccess")
	if r0, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, ActivitySuccess, 1, 2).Get(ctx); err != nil {
		logger.Debug("error getting activity success result", "err", err)
	} else {
		logger.Debug("ActivitySuccess result:", "r0", r0)
	}

	logger.Debug("Run SubWorkflow: Workflow2")
	f := workflow.CreateSubWorkflowInstance[string](ctx, workflow.SubWorkflowOptions{
		InstanceID: uuid.NewString(),
	}, Workflow2, "hello sub")

	workflow.Select(ctx,
		workflow.Await(f, func(ctx workflow.Context, f workflow.Future[string]) {
			rw, err := f.Get(ctx)
			if err != nil {
				logger.Debug("error getting workflow2 result", "err", err)
			} else {
				logger.Debug("Workflow2 result:", "rw", rw)
			}
		}),
	)

	logger.Debug("schedule ActivitySkip")
	if r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, ActivitySkip, 1, 2).Get(ctx); err != nil {
		logger.Debug("error getting activity skip result", "err", err)
	} else {
		logger.Debug("ActivitySkip result:", "r2", r2)
	}

	logger.Debug("Workflow finished")
	return "result", nil
}

func Workflow2(ctx workflow.Context, msg string) (ret string, err error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow2", "msg", msg)
	defer logger.Debug("Leaving Workflow2")

	defer func() {
		if errors.Is(ctx.Err(), workflow.Canceled) {
			logger.Debug("Workflow2 was canceled")

			logger.Debug("Do cleanup")
			ctx := workflow.NewDisconnectedContext(ctx)
			if _, err := workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, ActivityCleanup).Get(ctx); err != nil {
				panic("could not execute cleanup activity")
			}
			logger.Debug("Done with cleanup")

			ret = "cleanup result"
		}
	}()

	logger.Debug("schedule ActivityCancel")
	if r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, ActivityCancel, 1, 2).Get(ctx); err != nil {
		logger.Debug("error getting activity cancel result", "err", err)
	} else {
		logger.Debug("ActivityCancel result:", "r1", r1)
	}

	return "some result", nil
}

func ActivitySuccess(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivitySuccess")
	defer log.Println("Leaving ActivitySuccess")

	return a + b, nil
}

func ActivityCancel(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivityCancel")
	defer log.Println("Leaving ActivityCancel")

	// Wait for 10s, this will cause the cancellation event to be fired while waiting here
	time.Sleep(10 * time.Second)

	return a + b, nil
}

func ActivitySkip(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivitySkip")
	defer log.Println("Leaving ActivitySkip")

	return a + b, nil
}

func ActivityCleanup(ctx context.Context) error {
	log.Println("Entering ActivityCleanup")
	defer log.Println("Leaving ActivityCleanup")

	log.Println("Do some cleanup")

	return nil
}
