package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	b := sqlite.NewInMemoryBackend()
	b, err := redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)
	if err != nil {
		panic(err)
	}

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2

	cancel()
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) (string, error) {
	samples.Trace(ctx, "Entering Workflow1, input: ", msg)

	defer func() {
		samples.Trace(ctx, "Leaving Workflow1")
	}()

	a1 := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12)

	tctx, _ := workflow.WithCancel(ctx)
	t := workflow.ScheduleTimer(tctx, 2*time.Second)
	// cancel()

	workflow.Select(
		ctx,
		workflow.Await(t, func(ctx workflow.Context, f workflow.Future[struct{}]) {
			if _, err := f.Get(ctx); err != nil {
				samples.Trace(ctx, "Timer canceled")
			} else {
				samples.Trace(ctx, "Timer fired")
			}
		}),
		workflow.Await(a1, func(ctx workflow.Context, f workflow.Future[int]) {
			r, err := f.Get(ctx)
			if err != nil {
				panic(err)
			}

			samples.Trace(ctx, "Activity result", r)

			// Cancel timer
			// cancel()
		}),
	)

	return "result", nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")

	time.Sleep(10 * time.Second)

	defer func() {
		log.Println("Leaving Activity1")
	}()

	return a + b, nil
}
