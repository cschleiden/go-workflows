package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cschleiden/go-workflows/pkg/backend"
	"github.com/cschleiden/go-workflows/pkg/backend/mysql"
	"github.com/cschleiden/go-workflows/pkg/worker"
	scale "github.com/cschleiden/go-workflows/samples/scale"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	//b := sqlite.NewSqliteBackend("../scale.sqlite?_busy_timeout=10000")
	b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "simple")

	// Run worker
	go RunWorker(ctx, b)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2

	log.Println("Shutting down")
	cancel()
	time.Sleep(5 * time.Second)
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(scale.Workflow1)

	w.RegisterActivity(scale.Activity1)
	w.RegisterActivity(scale.Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
