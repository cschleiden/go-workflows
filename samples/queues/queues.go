package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
)

var (
	CustomActivityQueue = workflow.Queue("custom-activity-queue")
	CustomWorkflowQueue = workflow.Queue("custom-workflow-queue")
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	b := samples.GetBackend("queues", backend.WithLogger(slog.Default()))

	db, ok := b.(diag.Backend)
	if !ok {
		panic("backend does not implement diag.Backend")
	}

	// Start diagnostic server under /diag
	m := http.NewServeMux()
	m.Handle("/diag/", http.StripPrefix("/diag", diag.NewServeMux(db)))
	go func() {
		if err := http.ListenAndServe(":3000", m); err != nil {
			panic(err)
		}
	}()

	// Run worker
	w := RunDefaultWorker(ctx, b)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(SubWorkflow)
	w.RegisterActivity(Activity1)

	// This worker won't actually execute Activity2, but it still needs to be aware of its signature
	// since the workflow processed by this worker will schedule it.
	w.RegisterActivity(Activity2)

	activityWorker := worker.NewActivityWorker(b, &worker.ActivityWorkerOptions{
		ActivityPollers:          1,
		MaxParallelActivityTasks: 1,
		ActivityQueues:           []workflow.Queue{CustomActivityQueue},
	})

	activityWorker.RegisterActivity(Activity2)

	activityWorker.Start(ctx)

	workflowWorker := worker.NewWorkflowWorker(b, &worker.WorkflowWorkerOptions{
		WorkflowPollers:          1,
		MaxParallelWorkflowTasks: 1,
		WorkflowQueues:           []workflow.Queue{CustomWorkflowQueue},
	})

	workflowWorker.RegisterWorkflow(SubWorkflow)
	workflowWorker.RegisterActivity(Activity2)

	workflowWorker.Start(ctx)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	// Start workflow via client
	c := client.New(b)

	runWorkflow(ctx, c)

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	<-sigint

	cancel()

	if err := w.WaitForCompletion(); err != nil {
		panic("could not stop worker" + err.Error())
	}

	if err := activityWorker.WaitForCompletion(); err != nil {
		panic("could not stop activity worker" + err.Error())
	}
}

func runWorkflow(ctx context.Context, c *client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world"+uuid.NewString(), 42, Inputs{
		Msg:   "",
		Times: 0,
	})
	if err != nil {
		log.Fatal(err)
		panic("could not start workflow")
	}

	result, err := client.GetWorkflowResult[int](ctx, c, wf, time.Second*10)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Workflow finished. Result:", result)
}

func RunDefaultWorker(ctx context.Context, mb backend.Backend) *worker.Worker {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	return w
}
