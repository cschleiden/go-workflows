package main

import (
	"context"
	"log"

	"github.com/cschleiden/go-workflows/pkg/backend/mysql"
	"github.com/cschleiden/go-workflows/pkg/client"
	simple_split_worker "github.com/cschleiden/go-workflows/samples/simple-split-worker"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	//b := sqlite.NewSqliteBackend("../simple-split.sqlite")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "simple")

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

	log.Println("Started workflow", wf.GetInstanceID())
}
