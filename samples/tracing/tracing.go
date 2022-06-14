package main

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/client"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

	"github.com/cschleiden/go-workflows/worker"

	"github.com/google/uuid"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	r := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("go-workflows sample"),
		semconv.ServiceVersionKey.String("v0.1.0"),
		attribute.String("environment", "sample"),
	)

	stdoutexp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(err)
	}

	oclient := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("localhost:8360"), otlptracehttp.WithURLPath("/traces/otlp/v0.9"), otlptracehttp.WithInsecure())
	exp, err := otlptrace.New(ctx, oclient)
	if err != nil {
		panic(err)
	}

	tp := trace.NewTracerProvider(
		trace.WithSyncer(stdoutexp),
		trace.WithBatcher(exp),
		trace.WithResource(r),
	)

	otel.SetTracerProvider(tp)

	// b := sqlite.NewInMemoryBackend(backend.WithTracerProvider(tp))

	b, err := redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0, redis.WithBackendOptions(backend.WithTracerProvider(tp)))
	if err != nil {
		panic(err)
	}

	// Run worker
	w := RunWorker(ctx, b)

	c := client.New(b)

	runWorkflow(ctx, c)

	cancel()

	if err := w.WaitForCompletion(); err != nil {
		panic("could not stop worker" + err.Error())
	}

	tp.Shutdown(context.Background())
}

func runWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world"+uuid.NewString(), 42, Inputs{
		Msg:   "",
		Times: 0,
	})
	if err != nil {
		log.Fatal(err)
		panic("could not start workflow")
	}

	// Test signal
	time.Sleep(time.Millisecond * 500)
	c.SignalWorkflow(ctx, wf.InstanceID, "test-signal", "")

	result, err := client.GetWorkflowResult[int](ctx, c, wf, time.Second*120)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Workflow finished. Result:", result)
}

func RunWorker(ctx context.Context, mb backend.Backend) worker.Worker {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(Subworkflow)

	w.RegisterActivity(Activity1)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	return w
}
