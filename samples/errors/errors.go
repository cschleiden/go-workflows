package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/memory"
	"github.com/cschleiden/go-dt/pkg/client"
	"github.com/cschleiden/go-dt/pkg/worker"
	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/google/uuid"
	errs "github.com/pkg/errors"
)

func main() {
	ctx := context.Background()

	// b := sqlite.NewSqliteBackend("simple.sqlite")
	b := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.NewClient(b)

	startWorkflow(ctx, c)
	// startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
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
	w := worker.NewWorker(mb)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) error {
	log.Println("Entering Workflow1")
	log.Println("\tWorkflow instance input:", msg)
	log.Println("\tIsReplaying:", workflow.Replaying(ctx))
	defer func() { log.Println("Leaving Workflow1") }()

	a1 := workflow.ExecuteActivity(ctx, Activity1, 35, 12)

	var r1 int
	err := a1.Get(ctx, &r1)
	if err != nil {
		log.Println("Error from Activity 1", err)
		return errs.Wrap(err, "error getting result from activity 1")
	}
	log.Println("R1 result:", r1)
	log.Println("\tIsReplaying:", workflow.Replaying(ctx))

	log.Println("Completing workflow 1")
	return nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")
	defer func() { log.Println("Leaving Activity1") }()

	return 0, errors.New("some activity error")
}
