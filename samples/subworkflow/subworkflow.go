package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	b := samples.GetBackend("subworkflow")

	// Run worker
	w := RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	cancel()

	if err := w.WaitForCompletion(); err != nil {
		panic("could not stop worker" + err.Error())
	}
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, ParentWorkflow, "Hello world"+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)

	result, err := client.GetWorkflowResult[any](ctx, c, wf, time.Second*200)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Workflow finished. Result:", result)
}

func RunWorker(ctx context.Context, mb backend.Backend) worker.Worker {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(ParentWorkflow)
	w.RegisterWorkflow(SubWorkflow)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	return w
}

func ParentWorkflow(ctx workflow.Context, msg string) error {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1")
	logger.Debug("\tWorkflow instance input:", "msg", msg)

	wr := workflow.CreateSubWorkflowInstance[string](ctx, workflow.DefaultSubWorkflowOptions, SubWorkflow, "some input")

	result, err := wr.Get(ctx)
	if err != nil && !errors.Is(err, workflow.Canceled) {
		return fmt.Errorf("getting sub workflow result: %w", err)
	}

	logger.Debug("Sub workflow result:", "result", result)

	return nil
}

func SubWorkflow(ctx workflow.Context, msg string) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering SubWorkflow", "msg", msg)
	defer logger.Debug("Leaving SubWorkflow")

	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		logger.Error("error getting activity 1 result", "err", err)
	}
	logger.Debug("R1 result:", "r1", r1)

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		logger.Error("error getting activity 2 result", "err", err)
	}
	logger.Debug("R2 result:", "r2", r2)

	return "W2 Result", nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")
	defer log.Println("Leaving Activity2")

	return 12, nil
}
