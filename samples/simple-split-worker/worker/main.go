package main

import (
	"context"
	"os"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/sqlite"
	"github.com/cschleiden/go-dt/pkg/worker"
	simple_split_worker "github.com/cschleiden/go-dt/samples/simple-split-worker"
)

func main() {
	ctx := context.Background()

	mb := sqlite.NewSqliteBackend("../simple-split.sqlite")

	// Run worker
	go RunWorker(ctx, mb)

	c2 := make(chan os.Signal, 1)
	<-c2
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb)

	w.RegisterWorkflow(simple_split_worker.Workflow1)

	w.RegisterActivity(simple_split_worker.Activity1)
	w.RegisterActivity(simple_split_worker.Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
