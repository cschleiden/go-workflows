# 🔄 go-workflows

[![Build & Test](https://github.com/cschleiden/go-workflows/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/cschleiden/go-workflows/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/cschleiden/go-workflows.svg)](https://pkg.go.dev/github.com/cschleiden/go-workflows)
[![Go Report Card](https://goreportcard.com/badge/github.com/cschleiden/go-workflows)](https://goreportcard.com/report/github.com/cschleiden/go-workflows)

**A powerful, embedded workflow orchestration engine for Go applications.**

go-workflows enables you to build resilient, long-running processes with automatic retries, persistence, and monitoring. Write complex business logic as simple Go code that can survive crashes, restarts, and failures.

## ✨ Key Features

- 🏗️ **Native Go**: Write workflows and activities in plain Go code
- 🔄 **Automatic Recovery**: Workflows resume exactly where they left off after crashes
- 🏪 **Multiple Backends**: SQLite, MySQL, Redis, and in-memory storage options  
- ⚡ **High Performance**: Built for speed with efficient execution and minimal overhead
- 🔍 **Observability**: Built-in tracing, metrics, and diagnostic web UI
- 🧪 **Testing Support**: Comprehensive testing utilities with time manipulation
- 🎯 **Type Safety**: Full Go generics support for compile-time type checking

## 🚀 Quick Start

### Installation

```bash
go get github.com/cschleiden/go-workflows
```

### Your First Workflow

Create a simple workflow that processes data through multiple steps:

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/cschleiden/go-workflows/backend/sqlite"
    "github.com/cschleiden/go-workflows/client"
    "github.com/cschleiden/go-workflows/workflow"
    "github.com/cschleiden/go-workflows/worker"
    "github.com/google/uuid"
)

// Define your workflow
func ProcessOrderWorkflow(ctx workflow.Context, orderID string) (string, error) {
    logger := workflow.Logger(ctx)
    logger.Info("Processing order", "orderID", orderID)
    
    // Execute activities in sequence
    payment, err := workflow.ExecuteActivity[string](ctx, 
        workflow.DefaultActivityOptions, ProcessPayment, orderID).Get(ctx)
    if err != nil {
        return "", err
    }
    
    shipment, err := workflow.ExecuteActivity[string](ctx,
        workflow.DefaultActivityOptions, ShipOrder, orderID, payment).Get(ctx)
    if err != nil {
        return "", err
    }
    
    return shipment, nil
}

// Define your activities
func ProcessPayment(ctx context.Context, orderID string) (string, error) {
    // Simulate payment processing
    time.Sleep(100 * time.Millisecond)
    return "payment-" + orderID, nil
}

func ShipOrder(ctx context.Context, orderID, paymentID string) (string, error) {
    // Simulate shipping
    time.Sleep(100 * time.Millisecond)
    return "shipment-" + orderID, nil
}

func main() {
    ctx := context.Background()
    
    // Setup backend (SQLite for this example)
    backend := sqlite.NewSqliteBackend("orders.db")
    
    // Start worker
    w := worker.New(backend, nil)
    w.RegisterWorkflow(ProcessOrderWorkflow)
    w.RegisterActivity(ProcessPayment)
    w.RegisterActivity(ShipOrder)
    
    go func() {
        if err := w.Start(ctx); err != nil {
            log.Fatal("Failed to start worker:", err)
        }
    }()
    
    // Create and run workflow
    c := client.New(backend)
    wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
        InstanceID: uuid.NewString(),
    }, ProcessOrderWorkflow, "order-123")
    
    if err != nil {
        log.Fatal("Failed to start workflow:", err)
    }
    
    // Wait for result
    result, err := client.GetWorkflowResult[string](ctx, c, wf, time.Second*30)
    if err != nil {
        log.Fatal("Workflow failed:", err)
    }
    
    log.Printf("Order processed successfully: %s", result)
}
```

### Run the Example

```bash
go mod init my-workflow-app
go get github.com/cschleiden/go-workflows
go run main.go
```

## 🎯 Why go-workflows?

**Resilient by Design**: Traditional applications lose state when they crash. go-workflows automatically persists execution state, allowing complex processes to survive failures and resume seamlessly.

**Simple Mental Model**: Write business logic as straightforward Go functions. The framework handles the complexity of state management, retries, and recovery.

**Production Ready**: Battle-tested patterns inspired by [Temporal](https://temporal.io) and [Azure Durable Functions](https://docs.microsoft.com/en-us/azure/azure-functions/durable/), adapted for the Go ecosystem.

## 📖 Documentation

- **[Complete Documentation](http://cschleiden.github.io/go-workflows)** - Comprehensive guides and API reference
- **[Blog Series](https://cschleiden.dev/blog/2022-02-13-go-workflows-part1/)** - Deep dives into concepts and patterns
- **[Examples](./samples/)** - Ready-to-run sample applications

## 🏛️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│     Client      │    │     Worker       │    │    Backend      │
│                 │    │                  │    │                 │
│  ┌─────────────┐│    │ ┌──────────────┐ │    │ ┌─────────────┐ │
│  │  Workflow   ││───▶│ │   Workflow   │ │───▶│ │   History   │ │
│  │  Instances  ││    │ │   Executor   │ │    │ │   Storage   │ │
│  └─────────────┘│    │ └──────────────┘ │    │ └─────────────┘ │
│                 │    │                  │    │                 │
│  ┌─────────────┐│    │ ┌──────────────┐ │    │ ┌─────────────┐ │
│  │   Signals   ││───▶│ │   Activity   │ │───▶│ │    Queue    │ │
│  │             ││    │ │   Executor   │ │    │ │  Management │ │
│  └─────────────┘│    │ └──────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 📚 Learn More

**Core Concepts**:
- **[Workflows](http://cschleiden.github.io/go-workflows/#workflow)** - Deterministic functions that coordinate activities
- **[Activities](http://cschleiden.github.io/go-workflows/#activities)** - Units of work that can have side effects  
- **[Workers](http://cschleiden.github.io/go-workflows/#workers)** - Processes that execute workflows and activities
- **[Backends](http://cschleiden.github.io/go-workflows/#backends)** - Pluggable storage engines

**Advanced Features**:
- [Sub-workflows](./samples/subworkflow/) - Compose complex workflows from simpler ones
- [Signals](./samples/signal/) - Send messages to running workflows
- [Timers](./samples/timer/) - Schedule delayed execution
- [Error Handling](./samples/errors/) - Robust error handling and retries
- [Testing](./samples/) - Comprehensive testing utilities

## 🧩 Sample Applications

Explore real-world examples in the [`samples/`](./samples/) directory:

| Sample | Description |
|--------|-------------|
| [**Simple**](./samples/simple/) | Basic workflow with sequential activities |
| [**Web UI**](./samples/web/) | Interactive diagnostic dashboard |
| [**Concurrent**](./samples/concurrent/) | Parallel activity execution |
| [**Sub-workflows**](./samples/subworkflow/) | Composing workflows |
| [**Signals**](./samples/signal/) | External workflow communication |
| [**Timers**](./samples/timer/) | Scheduled and delayed execution |
| [**Error Handling**](./samples/errors/) | Robust error handling patterns |

### Try the Diagnostic Web UI

```bash
cd samples/web
go run .
# Visit http://localhost:3000/diag
```

<details>
<summary>🎮 Interactive Examples</summary>

**Run with SQLite** (no dependencies):
```bash
cd samples/simple
go run . -backend sqlite
```

**Run with Redis** (requires Docker):
```bash
docker compose up -d  # Start Redis
cd samples/simple  
go run .             # Uses Redis by default
```

**Benchmark Performance**:
```bash
cd bench
go run . -help       # See all options
go run . -runs 10 -fanout 3 -depth 2
```

</details>

## 🏪 Backends

Choose the storage backend that fits your needs:

| Backend | Use Case | Setup |
|---------|----------|-------|
| **SQLite** | Development, embedded apps | `sqlite.NewSqliteBackend("app.db")` |
| **MySQL** | Production, shared state | `mysql.NewMysqlBackend(connectionString)` |
| **Redis** | High performance, caching | `redis.NewRedisBackend(redisClient)` |
| **In-Memory** | Testing, ephemeral workloads | `monoprocess.NewMonoprocessBackend()` |

## 🛠️ Advanced Usage

### WorkflowOrchestrator (All-in-One)

For simpler scenarios, combine client and worker in a single component:

```go
func main() {
    ctx := context.Background()
    backend := sqlite.NewSqliteBackend("simple.db")
    
    orchestrator := worker.NewOrchestrator(backend, nil)
    orchestrator.RegisterWorkflow(MyWorkflow)
    orchestrator.RegisterActivity(MyActivity)
    
    if err := orchestrator.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Execute workflow
    result, err := orchestrator.ExecuteWorkflow(ctx, 
        client.WorkflowInstanceOptions{}, MyWorkflow, "input")
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Result: %v", result)
}
```

### Production Deployment

**Separate Client and Worker Processes**:

```go
// Worker process
func main() {
    backend := mysql.NewMysqlBackend(connectionString)
    worker := worker.New(backend, &worker.Options{
        MaxParallelWorkflows: 100,
        MaxParallelActivities: 1000,
    })
    
    // Register all workflows and activities
    worker.RegisterWorkflow(OrderProcessing)
    worker.RegisterActivity(ChargeCard)
    worker.RegisterActivity(SendEmail)
    
    worker.Start(context.Background())
}
```

```go  
// Client process
func main() {
    backend := mysql.NewMysqlBackend(connectionString)
    client := client.New(backend)
    
    // Start workflows as needed
    wf, err := client.CreateWorkflowInstance(ctx, 
        client.WorkflowInstanceOptions{
            InstanceID: userID,
            Queue: "orders",
        }, OrderProcessing, orderData)
}
```

## 🔍 Monitoring & Observability

### Built-in Web UI

The diagnostic UI provides real-time workflow monitoring:

```go
import "github.com/cschleiden/go-workflows/diag"

mux := http.NewServeMux()
mux.Handle("/diag/", http.StripPrefix("/diag", diag.NewServeMux(backend)))
go http.ListenAndServe(":3000", mux)
```

### OpenTelemetry Integration

```go
import "go.opentelemetry.io/otel"

backend := sqlite.NewSqliteBackend("app.db") 
worker := worker.New(backend, &worker.Options{
    Tracer: otel.Tracer("my-workflows"),
})
```

## 🧪 Testing

go-workflows includes powerful testing utilities:

```go
import "github.com/cschleiden/go-workflows/tester"

func TestMyWorkflow(t *testing.T) {
    tester := tester.NewWorkflowTester[string](MyWorkflow)
    
    // Mock activities
    tester.OnActivity(SendEmail, mock.Anything).Return("sent", nil)
    
    // Execute workflow
    result, err := tester.Execute("test-input")
    
    assert.NoError(t, err)
    assert.Equal(t, "expected-result", result)
}
```

**Time Manipulation for Testing**:
```go
tester.SetTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
tester.Execute("input")

// Advance time to trigger timers
tester.SetTime(time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC))
```

## 🚀 Production Considerations

- **Scaling**: Run multiple worker processes for horizontal scaling
- **Monitoring**: Use OpenTelemetry for distributed tracing
- **Persistence**: Choose appropriate backend for your durability requirements  
- **Testing**: Use the built-in testing framework for workflow validation
- **Deployment**: Consider separate client/worker deployments for flexibility

## 🤝 Contributing

We welcome contributions! Please see our [development guide](./DEVELOPMENT.md) for:

- Setting up the development environment
- Running tests across multiple backends  
- Code style and linting guidelines
- Submitting pull requests

## 📄 License

Licensed under the [MIT License](./LICENSE).

---

**Inspired by**: [Temporal](https://github.com/temporalio/temporal) • [Cadence](https://github.com/uber/cadence) • [Azure Durable Task Framework](https://github.com/Azure/durabletask)
```
