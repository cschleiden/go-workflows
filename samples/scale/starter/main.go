package main

import (
	"context"
	"log"

	"github.com/cschleiden/go-dt/pkg/backend/sqlite"
	"github.com/cschleiden/go-dt/pkg/client"
	scale "github.com/cschleiden/go-dt/samples/scale"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	mb := sqlite.NewSqliteBackend("../scale.sqlite")

	// Start workflow via client
	c := client.New(mb)

	for i := 0; i < 100; i++ {
		startWorkflow(ctx, c)
	}
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, scale.Workflow1, "Hello world "+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.GetInstanceID())
}
