package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// b := sqlite.NewInMemoryBackend()
	// b := sqlite.NewSqliteBackend("subworkflow.sqlite")
	// b := mysql.NewMysqlBackend("localhost", 3306, "root", "root", "simple")
	b, err := redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)
	if err != nil {
		panic(err)
	}

	// Run worker
	w := RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	cancel()

	if err := w.Stop(); err != nil {
		panic("could not stop worker" + err.Error())
	}
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world"+uuid.NewString())
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)

	result, err := client.GetWorkflowResult[any](ctx, c, wf, time.Second*10)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Workflow finished. Result:", result)
}

func RunWorker(ctx context.Context, mb backend.Backend) worker.Worker {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(Workflow2)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	return w
}

func Workflow1(ctx workflow.Context, msg string) error {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1")
	logger.Debug("\tWorkflow instance input:", "msg", msg)

	wr, err := workflow.CreateSubWorkflowInstance[string](ctx, workflow.DefaultSubWorkflowOptions, Workflow2, "some input").Get(ctx)
	if err != nil {
		return fmt.Errorf("getting sub workflow result: %w", err)
	}

	logger.Debug("Sub workflow result:", wr)

	return nil
}

func Workflow2(ctx workflow.Context, msg string) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow2")
	logger.Debug("\tWorkflow instance input:", "msg", msg)

	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	logger.Debug("R1 result:", r1)

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	logger.Debug("R2 result:", r2)

	return "W2 Result", nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")
	defer log.Println("Leaving Activity2")

	return 12, nil
}
