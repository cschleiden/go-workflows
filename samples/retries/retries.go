package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/sqlite"
	"github.com/cschleiden/go-dt/pkg/client"
	"github.com/cschleiden/go-dt/pkg/worker"
	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/cschleiden/go-dt/samples"
	"github.com/google/uuid"
	errs "github.com/pkg/errors"
)

func main() {
	ctx := context.Background()

	// b := sqlite.NewSqliteBackend("simple.sqlite")
	b := sqlite.NewInMemoryBackend()

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)
	// startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world"+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.GetInstanceID())
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(WorkflowWithFailures)

	w.RegisterActivity(Activity1)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) error {
	trace(ctx, "Entering Workflow1", msg)
	defer trace(ctx, "Leaving Workflow1")

	// Illustrate sub workflow retries. The called workflow will fail a few times, and its execution will be retried.
	err := workflow.CreateSubWorkflowInstance(ctx, workflow.SubWorkflowOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts:        3,
			FirstRetryInterval: time.Second * 3,
			BackoffCoefficient: 1,
		},
	}, WorkflowWithFailures, "Hello world"+uuid.NewString()).Get(ctx, nil)
	if err != nil {
		return errs.Wrap(err, "error starting subworkflow")
	}

	trace(ctx, "Completing workflow 1")
	return nil
}

var workflowCalls = 0

func WorkflowWithFailures(ctx workflow.Context, msg string) error {
	trace(ctx, "Entering WorkflowWithFailures", msg)
	defer trace(ctx, "Leaving WorkflowWithFailures")

	workflowCalls++
	if workflowCalls < 3 {
		return fmt.Errorf("workflow error %d", workflowCalls)
	}

	// Illustrate activity retries. The called activity will fail a few times, and its execution will be retried.
	var r1 int
	err := workflow.ExecuteActivity(ctx, workflow.ActivityOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts:        3,
			FirstRetryInterval: time.Second * 3,
			BackoffCoefficient: 2,
		},
	}, Activity1, 35, 12).Get(ctx, &r1)
	if err != nil {
		trace(ctx, "Error from Activity 1", err)
		return errs.Wrap(err, "error getting result from activity 1")
	}

	trace(ctx, "R1 result:", r1)

	trace(ctx, "Completing workflow 1")
	return nil
}

var activityCalls = 0

func Activity1(ctx context.Context, a int) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	activityCalls++
	if activityCalls < 3 {
		time.Sleep(2 * time.Second)

		log.Println(">> Activity error", activityCalls)
		return 0, fmt.Errorf("activity error %d", activityCalls)
	}

	return a, nil
}

func trace(ctx workflow.Context, args ...interface{}) {
	samples.Trace(ctx, args...)
}
