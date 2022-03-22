package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/backend/mysql"
	"github.com/cschleiden/go-workflows/client"
	scale "github.com/cschleiden/go-workflows/samples/scale"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	now := time.Now()

	//b := sqlite.NewSqliteBackend("../scale.sqlite")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "scale")

	// Start workflow via client
	c := client.New(b)

	w := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		w.Add(1)
		go startWorkflow(ctx, i, c, &w)
	}

	w.Wait()

	log.Println("Done after", time.Since(now))
}

func startWorkflow(ctx context.Context, i int, c client.Client, w *sync.WaitGroup) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, scale.Workflow1, "Hello world")
	if err != nil {
		log.Fatal("could not start workflow", err)
	}

	log.Println("Started workflow", i, wf.GetInstanceID())

	defer w.Done()
	if err := c.WaitForWorkflowInstance(ctx, wf, time.Second*30); err != nil {
		log.Println("Could not wait for workflow:", i, err)
	} else {
		log.Println("Finished workflow", i, wf.GetInstanceID())
	}
}
