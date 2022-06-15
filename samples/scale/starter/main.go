package main

import (
	"context"
	"flag"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	scale "github.com/cschleiden/go-workflows/samples/scale"
	"github.com/google/uuid"
)

var tostart = flag.Int("count", 100, "Number of workflow instances to start")
var backendType = flag.String("backend", "redis", "backend to use: sqlite, mysql, redis")
var count int32

func main() {
	flag.Parse()

	ctx := context.Background()

	count = int32(*tostart)

	b := samples.GetBackend("scale")

	// Start workflow via client
	c := client.New(b)

	wg := &sync.WaitGroup{}

	now := time.Now()

	for i := 0; i < *tostart; i++ {
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

	err = c.WaitForWorkflowInstance(ctx, wf, time.Second*30)
	if err != nil {
		log.Println("Received error while waiting for workflow", wf.InstanceID, err)
	}

	cn := atomic.AddInt32(&count, -1)
	log.Print(cn)

	wg.Done()
}
