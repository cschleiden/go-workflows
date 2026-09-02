package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	postgres "github.com/cschleiden/go-workflows/backend/postgres"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

// This sample demonstrates the use of LISTEN/NOTIFY for reactive task polling
// in the Postgres backend, which allows workers to be notified immediately
// when new tasks are available instead of polling.

func main() {
	enableNotify := flag.Bool("notify", false, "enable LISTEN/NOTIFY for reactive polling")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create backend with optional LISTEN/NOTIFY enabled
	baseOpt := postgres.WithBackendOptions(backend.WithStickyTimeout(0))

	if *enableNotify {
		log.Println("✓ LISTEN/NOTIFY enabled for reactive polling")
		b := postgres.NewPostgresBackend("localhost", 5432, "root", "root", "postgres",
			baseOpt,
			postgres.WithNotifications(true))
		defer b.Close()
		runWorkflow(ctx, b, *enableNotify)
	} else {
		log.Println("✗ Using traditional polling (use -notify to enable LISTEN/NOTIFY)")
		b := postgres.NewPostgresBackend("localhost", 5432, "root", "root", "postgres", baseOpt)
		defer b.Close()
		runWorkflow(ctx, b, *enableNotify)
	}
}

func runWorkflow(ctx context.Context, b backend.Backend, notifyEnabled bool) {
	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create worker
	w := worker.New(b, nil)
	w.RegisterWorkflow(SimpleWorkflow)
	w.RegisterActivity(SimpleActivity)

	if err := w.Start(innerCtx); err != nil {
		log.Fatal("could not start worker:", err)
	}

	// Create client
	c := client.New(b)

	// Run workflow
	log.Println("Starting workflow...")
	start := time.Now()

	wf, err := c.CreateWorkflowInstance(innerCtx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, SimpleWorkflow, "test")
	if err != nil {
		log.Fatal("could not start workflow:", err)
	}

	result, err := client.GetWorkflowResult[string](innerCtx, c, wf, time.Second*10)
	if err != nil {
		log.Fatal("workflow execution failed:", err)
	}

	elapsed := time.Since(start)
	log.Printf("✓ Workflow completed in %v: %s", elapsed, result)

	if notifyEnabled {
		log.Println("With LISTEN/NOTIFY, the worker was notified immediately when tasks became available.")
	} else {
		log.Println("Without LISTEN/NOTIFY, the worker had to poll for tasks.")
	}

	cancel()
	if err := w.WaitForCompletion(); err != nil {
		log.Fatal("error stopping worker:", err)
	}
}

func SimpleWorkflow(ctx workflow.Context, input string) (string, error) {
	r1, err := workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, SimpleActivity, input).Get(ctx)
	if err != nil {
		return "", err
	}

	return r1, nil
}

func SimpleActivity(ctx context.Context, input string) (string, error) {
	return "Processed: " + input, nil
}
