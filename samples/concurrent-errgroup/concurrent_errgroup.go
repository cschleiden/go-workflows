package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("concurrent-errgroup", true)

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func startWorkflow(ctx context.Context, c *client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, WorkflowErrGroup, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(WorkflowErrGroup)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

// WorkflowErrGroup demonstrates running two concurrent branches using the workflow-native
// error group. If any branch returns an error, the group's context is canceled and the
// first error is returned from Wait.
func WorkflowErrGroup(ctx workflow.Context, msg string) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering WorkflowErrGroup")
	logger.Debug("\tWorkflow instance input:", "msg", msg)

	defer func() {
		logger.Debug("Leaving WorkflowErrGroup")
	}()

	gctx, g := workflow.WithErrGroup(ctx)

	g.Go(func(ctx workflow.Context) error {
		a1 := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12)
		r, err := a1.Get(ctx)
		if err != nil {
			return err
		}

		logger.Debug("A1 result", "r", r)
		return nil
	})

	g.Go(func(ctx workflow.Context) error {
		a2 := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2)
		r, err := a2.Get(ctx)
		if err != nil {
			return err
		}

		logger.Debug("A2 result", "r", r)
		return nil
	})

	// Wait for both goroutines to finish and return the first error, if any
	if err := g.Wait(gctx); err != nil {
		return "", err
	}

	return "result", nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")

	defer func() {
		log.Println("Leaving Activity1")
	}()

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")

	time.Sleep(5 * time.Second)

	defer func() {
		log.Println("Leaving Activity2")
	}()

	return 12, nil
}
