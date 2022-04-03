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

	//b := sqlite.NewSqliteBackend("../scale.sqlite")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "scale")

	// Start workflow via client
	c := client.New(b)

	wg := &sync.WaitGroup{}

	now := time.Now()

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go startWorkflow(ctx, c, wg)
	}

	wg.Wait()

	log.Println("Finished in", time.Since(now))
}

func startWorkflow(ctx context.Context, c client.Client, wg *sync.WaitGroup) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, scale.Workflow1, "Hello world "+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	// log.Println("Started workflow", wf.GetInstanceID())

	c.WaitForWorkflowInstance(ctx, wf, time.Second*30)

	wg.Done()
}
