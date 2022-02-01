package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/mysql"
	"github.com/cschleiden/go-dt/pkg/client"
	"github.com/cschleiden/go-dt/pkg/worker"
	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/cschleiden/go-dt/samples"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	//b := memory.NewMemoryBackend()
	//b := sqlite.NewSqliteBackend("cancellation.sqlite?ephemeral=true")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "cancellation")

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	<-c2

	cancel()
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.GetInstanceID())

	time.Sleep(2 * time.Second)

	err = c.CancelWorkflowInstance(ctx, wf)
	if err != nil {
		panic("could not cancel workflow")
	}

	log.Println("Cancelled workflow", wf.GetInstanceID())
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb)

	w.RegisterWorkflow(Workflow1)
	w.RegisterActivity(ActivityCancel)
	w.RegisterActivity(ActivitySkip)
	w.RegisterActivity(ActivitySuccess)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) (string, error) {
	samples.Trace(ctx, "Entering Workflow1")
	defer samples.Trace(ctx, "Leaving Workflow1")
	samples.Trace(ctx, "\tWorkflow instance input:", msg)

	var r0 int
	samples.Trace(ctx, "schedule ActivitySuccess")
	if err := workflow.ExecuteActivity(ctx, ActivitySuccess, 1, 2).Get(ctx, &r0); err != nil {
		samples.Trace(ctx, "error getting activity success result", err)
	} else {
		samples.Trace(ctx, "ActivitySuccess result:", r0)
	}

	var r1 int
	samples.Trace(ctx, "schedule ActivityCancel")
	if err := workflow.ExecuteActivity(ctx, ActivityCancel, 1, 2).Get(ctx, &r1); err != nil {
		samples.Trace(ctx, "error getting activity cancel result", err)
	}
	samples.Trace(ctx, "ActivityCancel result:", r1)

	var r2 int
	samples.Trace(ctx, "schedule ActivitySkip")
	if err := workflow.ExecuteActivity(ctx, ActivitySkip, 1, 2).Get(ctx, &r2); err != nil {
		samples.Trace(ctx, "error getting activity skip result", err)
	}
	samples.Trace(ctx, "ActivitySkip result:", r2)

	samples.Trace(ctx, "Workflow finished")
	return "result", nil
}

func ActivitySuccess(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivitySuccess")
	defer log.Println("Leaving ActivitySuccess")

	return a + b, nil
}

func ActivityCancel(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivityCancel")
	defer log.Println("Leaving ActivityCancel")

	time.Sleep(10 * time.Second)

	return a + b, nil
}

func ActivitySkip(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivitySkip")
	defer log.Println("Leaving ActivitySkip")

	return a + b, nil
}
