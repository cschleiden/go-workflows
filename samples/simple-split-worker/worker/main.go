package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/cschleiden/go-workflows/pkg/backend"
	"github.com/cschleiden/go-workflows/pkg/backend/mysql"
	"github.com/cschleiden/go-workflows/pkg/worker"
	simple_split_worker "github.com/cschleiden/go-workflows/samples/simple-split-worker"
)

func main() {
	ctx := context.Background()

	//b := sqlite.NewSqliteBackend("../simple-split.sqlite")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "simple")

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
