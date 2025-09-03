package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/cschleiden/go-workflows/workflow/executor"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	r := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("go-workflows sample"),
		semconv.ServiceVersionKey.String("v0.1.0"),
		attribute.String("environment", "sample"),
	)

	oclient := otlptracehttp.NewClient(
		// otlptracehttp.WithEndpoint("localhost:8360"),
		// otlptracehttp.WithURLPath("/traces/otlp/v0.9"),
		otlptracehttp.WithEndpoint("localhost:4318"),
		otlptracehttp.WithInsecure(),
	)
	exp, err := otlptrace.New(ctx, oclient)
	if err != nil {
		panic(err)
	}

	stdoutexp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(err)
	}

	tp := trace.NewTracerProvider(
		trace.WithSyncer(stdoutexp),
		trace.WithBatcher(exp),
		trace.WithResource(r),
	)

	otel.SetTracerProvider(tp)

	b := samples.GetBackend("tracing",
		backend.WithTracerProvider(tp),
		backend.WithStickyTimeout(0),
	)

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

func runWorkflow(ctx context.Context, c *client.Client) {
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
	time.Sleep(time.Second * 5)
	c.SignalWorkflow(ctx, wf.InstanceID, "test-signal", "")

	result, err := client.GetWorkflowResult[int](ctx, c, wf, time.Second*120)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Workflow finished. Result:", result)
}

// Ensure we aren't caching for this sample
type noopWorkflowExecutorCache struct{}

var _ executor.Cache = (*noopWorkflowExecutorCache)(nil)

// Get implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) Get(ctx context.Context, instance *core.WorkflowInstance) (executor.WorkflowExecutor, bool, error) {
	return nil, false, nil
}

// Evict implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) Evict(ctx context.Context, instance *core.WorkflowInstance) error {
	return nil
}

// StartEviction implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) StartEviction(ctx context.Context) {
}

// Store implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) Store(ctx context.Context, instance *workflow.Instance, workflow executor.WorkflowExecutor) error {
	return nil
}

func RunWorker(ctx context.Context, mb backend.Backend) *worker.Worker {
	opt := worker.DefaultOptions
	opt.WorkflowExecutorCache = &noopWorkflowExecutorCache{}
	w := worker.New(mb, &opt)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(Subworkflow)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(RetriedActivity)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}

	return w
}
