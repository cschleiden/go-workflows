package main

import (
	"context"
	"errors"
	"log"

	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"
	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/client"
	"github.com/ticctech/go-workflows/samples"
	"github.com/ticctech/go-workflows/worker"
	"github.com/ticctech/go-workflows/workflow"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("activity-registration")

	go RunWorker(ctx, b)

	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func startWorkflow(ctx context.Context, c client.Client) {
	_, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world"+uuid.NewString())
	if err != nil {
		log.Panic(err)
	}
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)

	// Register activities with some shared state. SomeParam could be a data store,
	// a connection to another system, or anything that needs to be shared between
	// activities.
	//
	// State can be accessed in parallel and needs to be thread safe.
	w.RegisterActivity(&activities{SomeParam: "some value"})

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) error {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1")
	defer logger.Debug("Leaving Workflow1")

	var a *activities

	if r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a.Activity1, 35, 12, nil, "test").Get(ctx); err != nil {
		return errors.New("error getting activity 1 result")
	} else {
		logger.Debug("R1 result:", r1)
	}

	if r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a.Activity2).Get(ctx); err != nil {
		return errors.New("error getting activity 2 result")
	} else {
		logger.Debug("R2 result:", r2)
	}

	return nil
}

type activities struct {
	SomeParam string
}

func (a *activities) Activity1(ctx context.Context, x, y int) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	log.Println("Activity 1", a.SomeParam, x, y)

	time.Sleep(2 * time.Second)
	return x + y, nil
}

func (a *activities) Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")
	defer log.Println("Leaving Activity2")

	log.Println("Activity 2", a.SomeParam)

	time.Sleep(1 * time.Second)

	return 12, nil
}
