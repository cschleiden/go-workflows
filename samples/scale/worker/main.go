package main

import (
	"context"
	"os"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/sqlite"
	"github.com/cschleiden/go-dt/pkg/worker"
	scale "github.com/cschleiden/go-dt/samples/scale"
)

func main() {
	ctx := context.Background()

	mb := sqlite.NewSqliteBackend("../scale.sqlite?_busy_timeout=10000")

	// Run worker
	go RunWorker(ctx, mb)

	c2 := make(chan os.Signal, 1)
	<-c2
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.NewWorker(mb)

	w.RegisterWorkflow(scale.Workflow1)

	w.RegisterActivity(scale.Activity1)
	w.RegisterActivity(scale.Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
