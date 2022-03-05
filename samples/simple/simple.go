package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"

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
