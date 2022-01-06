package main

import (
	"context"
	"log"

	"github.com/cschleiden/go-dt/pkg/backend/sqlite"
	"github.com/cschleiden/go-dt/pkg/client"
	simple_split_worker "github.com/cschleiden/go-dt/samples/simple-split-worker"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	mb := sqlite.NewSqliteBackend("../simple-split.sqlite")

	// Start workflow via client
	c := client.NewClient(mb)
	startWorkflow(ctx, c)
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, simple_split_worker.Workflow1, "Hello world "+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.GetInstanceID())
}
