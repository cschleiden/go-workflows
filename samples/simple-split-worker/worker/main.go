package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/samples"
	simple_split_worker "github.com/cschleiden/go-workflows/samples/simple-split-worker"
	"github.com/cschleiden/go-workflows/worker"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("simple-split", false)

	// Run worker
	go RunWorker(ctx, b)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(simple_split_worker.Workflow1)

	w.RegisterActivity(simple_split_worker.Activity1)
	w.RegisterActivity(simple_split_worker.Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
