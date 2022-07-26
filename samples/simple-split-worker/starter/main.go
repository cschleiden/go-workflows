package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/ticctech/go-workflows/client"
	"github.com/ticctech/go-workflows/samples"
	simple_split_worker "github.com/ticctech/go-workflows/samples/simple-split-worker"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("simple-split")

	// Start workflow via client
	c := client.New(b)
	startWorkflow(ctx, c)
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, simple_split_worker.Workflow1, "Hello world "+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)
}
