package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/mysql"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	scale "github.com/cschleiden/go-workflows/samples/scale"
	"github.com/cschleiden/go-workflows/worker"
)

var backendType = flag.String("backend", "redis", "backend to use: sqlite, mysql, redis")

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	var b backend.Backend

	switch *backendType {
	case "sqlite":
		b = sqlite.NewSqliteBackend("../scale.sqlite?_busy_timeout=10000")

	case "mysql":
		b = mysql.NewMysqlBackend("localhost", 3306, "root", "root", "scale")

	case "redis":
		var err error
		b, err = redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)
		if err != nil {
			panic(err)
		}
	}

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
