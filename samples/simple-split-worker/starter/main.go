package main

import (
	"context"
	"log"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	simple_split_worker "github.com/cschleiden/go-workflows/samples/simple-split-worker"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("simple-split", false)

	// Start workflow via client
	c := client.New(b)
	startWorkflow(ctx, c)
}

func startWorkflow(ctx context.Context, c *client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, simple_split_worker.Workflow1, "Hello world "+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)
}
