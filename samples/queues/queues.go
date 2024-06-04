package main

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"

	"github.com/google/uuid"
)

var CustomActivityQueue = workflow.Queue("custom-activity-queue")

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	b := samples.GetBackend("queues", backend.WithLogger(slog.Default()))

	// Run worker
	w := RunDefaultWorker(ctx, b)

	w.RegisterWorkflow(Workflow1)
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

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	// Start workflow via client
	c := client.New(b)

	runWorkflow(ctx, c)

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
