package main

import (
	"context"
	"log"

	"github.com/cschleiden/go-workflows/backend/mysql"
	"github.com/cschleiden/go-workflows/client"
	scale "github.com/cschleiden/go-workflows/samples/scale"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	//b := sqlite.NewSqliteBackend("../scale.sqlite")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "simple")

	// Start workflow via client
	c := client.New(b)

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
