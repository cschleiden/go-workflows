package main

import (
	"context"
	"flag"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/client"
	scale "github.com/cschleiden/go-workflows/samples/scale"
	"github.com/google/uuid"
)

var tostart = flag.Int("count", 100, "Number of workflow instances to start")
var count int32

func main() {
	flag.Parse()

	count = int32(*tostart)

	ctx := context.Background()

	//b := sqlite.NewSqliteBackend("../scale.sqlite")
	// b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "scale")
	b, err := redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)
	if err != nil {
		panic(err)
	}

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

	// log.Println("Started workflow", wf.InstanceID)

	err = c.WaitForWorkflowInstance(ctx, wf, time.Second*30)
	if err != nil {
		log.Println("Received error while waiting for workflow", wf.InstanceID, err)
	}

	cn := atomic.AddInt32(&count, -1)
	log.Print(cn)

	wg.Done()
}
