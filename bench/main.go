package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/mysql"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	redisv8 "github.com/redis/go-redis/v9"
)

var b = flag.String("backend", "redis", "Backend to use. Supported backends are:\n- redis\n- mysql\n- sqlite\n")
var timeout = flag.Duration("timeout", time.Second*30, "Timeout for the benchmark run")
var scenario = flag.String("scenario", "basic", "Scenario to run. Support scenarios are:\n- basic\n")
var runs = flag.Int("runs", 1, "Number of root workflows to start")
var depth = flag.Int("depth", 2, "Depth of mid workflows")
var fanOut = flag.Int("fanout", 2, "Number of child workflows to execute per root/mid workflow")
var leafFanOut = flag.Int("leaffanout", 2, "Number of leaf workflows to execute per mid workflow")
var activities = flag.Int("activities", 2, "Number of activities to execute per leaf workflow")
var resultSize = flag.Int("resultsize", 100, "Size of activity result payload in bytes")
var format = flag.String("format", "text", "Output format. Supported formats are:\n- text\n- csv\n")
var cacheSize = flag.Int("cachesize", 128, "Size of the workflow executor cache")

func main() {
	flag.Parse()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(*timeout).Add(time.Second*5))
	defer cancel()

	mm := newMemMetrics()
	ba := getBackend(*b, backend.WithLogger(slog.New(&nullHandler{})), backend.WithMetrics(mm))

	wo := worker.DefaultWorkerOptions
	wo.WorkflowExecutorCacheSize = *cacheSize
	w := worker.New(ba, &wo)

	w.RegisterWorkflow(Root)
	w.RegisterWorkflow(Mid)
	w.RegisterWorkflow(Leaf)
	w.RegisterActivity(Activity)

	if err := w.Start(ctx); err != nil {
		panic(err)
	}

	c := client.New(ba)

	start := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < *runs; i++ {
		i, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
			InstanceID: fmt.Sprintf("root-%d", i),
		}, Root, &MidInput{
			FanOut: *fanOut,
			Depth:  *depth,

			LeafFanOut: *leafFanOut,

			Activities:       *activities,
			PayloadSizeBytes: *resultSize,
		})
		if err != nil {
			panic(err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := c.WaitForWorkflowInstance(ctx, i, *timeout)
			if err != nil {
				panic(fmt.Errorf("Workflow instance %s failed: %w", i.InstanceID, err))
			}
		}()
	}

	wg.Wait()

	end := time.Now()

	switch *format {
	case "text":
		log.Println("Ran", *runs, "root workflows in", end.Sub(start).Seconds(), "seconds")
		mm.Print()

	case "csv":
		fmt.Printf(
			"%s,%v,%s,%d,%d,%d,%d,%d,%d\n",
			*b, end.Sub(start).Seconds(), *scenario, *runs, *depth, *fanOut, *leafFanOut, *activities, *resultSize)
	}
}

func getBackend(b string, opt ...backend.BackendOption) backend.Backend {
	switch b {
	case "memory":
		return sqlite.NewInMemoryBackend(opt...)

	case "sqlite":
		os.Remove("bench.sqlite")

		return sqlite.NewSqliteBackend("bench.sqlite", opt...)

	case "mysql":
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", "root", "root"))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec("DROP DATABASE IF EXISTS bench"); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		if _, err := db.Exec("CREATE DATABASE bench"); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		if err := db.Close(); err != nil {
			panic(err)
		}

		return mysql.NewMysqlBackend("localhost", 3306, "root", "root", "bench", opt...)

	case "redis":
		rclient := redisv8.NewUniversalClient(&redisv8.UniversalOptions{
			Addrs:        []string{"localhost:6379"},
			Username:     "",
			Password:     "RedisPassw0rd",
			DB:           0,
			WriteTimeout: time.Second * 30,
			ReadTimeout:  time.Second * 30,
		})

		rclient.FlushAll(context.Background()).Result()

		b, err := redis.NewRedisBackend(rclient, redis.WithBackendOptions(opt...))
		if err != nil {
			panic(err)
		}

		return b

	default:
		panic("unknown backend " + b)
	}
}
