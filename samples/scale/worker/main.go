package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis"
	scale "github.com/cschleiden/go-workflows/samples/scale"
	"github.com/cschleiden/go-workflows/worker"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	//b := sqlite.NewSqliteBackend("../scale.sqlite?_busy_timeout=10000")
	// b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "scale")
	b := redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)

	// Run worker
	go RunWorker(ctx, b)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2

	log.Println("Shutting down")
	cancel()
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, &worker.Options{
		WorkflowPollers:          1,
		MaxParallelWorkflowTasks: 100,
		ActivityPollers:          1,
		MaxParallelActivityTasks: 100,
		HeartbeatWorkflowTasks:   false,
	})

	w.RegisterWorkflow(scale.Workflow1)

	w.RegisterActivity(scale.Activity1)
	w.RegisterActivity(scale.Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
