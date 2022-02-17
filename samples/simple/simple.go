package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"
cschleiden/go-workflows
	"github.com/cschleiden/go-workflows/pkg/backend"
	"github.com/cschleiden/go-workflows/pkg/backend/sqlite"
	"github.com/cschleiden/go-workflows/pkg/client"
	"github.com/cschleiden/go-workflows/pkg/worker"
	"github.com/cschleiden/go-workflows/pkg/workflow"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/google/uuid"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	b := sqlite.NewSqliteBackend("simple.sqlite")
	//b := sqlite.NewInMemoryBackend()
	//b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "simple")

	// Run worker
	w := RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2

	cancel()

	if err := w.Stop(); err != nil {
		panic("could not stop worker" + err.Error())
	}
}

func startWorkflow(ctx context.Context, c client.Client) {
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

	log.Println("Started workflow", wf.GetInstanceID())
}

func RunWorker(ctx context.Context, mb backend.Backend) worker.Worker {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	return w
}

type Inputs struct {
	Msg   string
	Times int
}

func Workflow1(ctx workflow.Context, msg string, times int, inputs Inputs) error {
	samples.Trace(ctx, "Entering Workflow1", msg, times, inputs)

	defer samples.Trace(ctx, "Leaving Workflow1")

	var r1 int
	err := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity1, 35, 12, nil, "test").Get(ctx, &r1)
	if err != nil {
		panic("error getting activity 1 result")
	}
	samples.Trace(ctx, "R1 result:", r1)

	var r2 int
	err = workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx, &r2)
	if err != nil {
		panic("error getting activity 1 result")
	}
	samples.Trace(ctx, "R2 result:", r2)

	return nil
}

func Activity1(ctx context.Context, a, b int, x, y *string) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	log.Println(x, *y)

	time.Sleep(5 * time.Second)

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")
	defer log.Println("Leaving Activity2")

	time.Sleep(1 * time.Second)

	return 12, nil
}
