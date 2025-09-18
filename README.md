<div align="center">
  <img src="docs/source/logo.png" alt="go-workflows" width="200"/>
  
  # go-workflows
  
  **Durable workflows for Go**
  
  [![Build & Test](https://github.com/cschleiden/go-workflows/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/cschleiden/go-workflows/actions/workflows/go.yml)
  [![Go Reference](https://pkg.go.dev/badge/github.com/cschleiden/go-workflows.svg)](https://pkg.go.dev/github.com/cschleiden/go-workflows)
  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
  [![Go Report Card](https://goreportcard.com/badge/github.com/cschleiden/go-workflows)](https://goreportcard.com/report/github.com/cschleiden/go-workflows)
</div>

---

## Overview

**go-workflows** is an embedded durable workflows engine for Go that orchestrates long-running processes with fault tolerance, state persistence, and automatic retries. Write complex business logic as simple, sequential Go code that survives failures and restarts.

Inspired by [Temporal](https://github.com/temporalio/temporal), [Cadence](https://github.com/uber/cadence), and [Azure Durable Task Framework](https://github.com/Azure/durabletask), go-workflows brings enterprise-grade workflow orchestration directly into your Go applications.

### ✨ Key Features

- **🔄 Durable Execution** - Workflows survive crashes, restarts, and failures
- **🏗️ Multiple Backends** - SQLite, MySQL, Redis, and in-memory storage
- **🎯 Type Safety** - Fully typed workflow and activity definitions
- **🔀 Complex Orchestration** - Sub-workflows, parallel execution, timers, and signals
- **📈 Observability** - Built-in OpenTelemetry tracing and metrics
- **🧪 Testing Framework** - Comprehensive testing utilities for workflows and activities
- **🚀 Zero Dependencies** - Embedded engine, no external services required

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [Examples](#examples)
- [Backends](#backends)
- [Advanced Features](#advanced-features)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

---

## Installation

```bash
go get github.com/cschleiden/go-workflows
```

**Requirements:** Go 1.24+

---

## Quick Start

Get started with go-workflows in minutes. Here's a complete example that demonstrates the core concepts:

### 1. Define a Workflow

Workflows are deterministic functions that orchestrate activities and maintain state:

```go
func OrderProcessingWorkflow(ctx workflow.Context, orderID string) error {
    // Validate order
    var order Order
    err := workflow.ExecuteActivity[Order](ctx, workflow.DefaultActivityOptions, 
        ValidateOrderActivity, orderID).Get(ctx, &order)
    if err != nil {
        return fmt.Errorf("order validation failed: %w", err)
    }

    // Process payment
    var paymentID string
    err = workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, 
        ProcessPaymentActivity, order.Total).Get(ctx, &paymentID)
    if err != nil {
        return fmt.Errorf("payment failed: %w", err)
    }

    // Ship order
    return workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, 
        ShipOrderActivity, orderID, paymentID).Get(ctx, nil)
}
```

### 2. Define Activities

Activities handle side effects and external system interactions:

```go
func ValidateOrderActivity(ctx context.Context, orderID string) (Order, error) {
    // Database lookup, API calls, etc.
    return Order{ID: orderID, Total: 99.99}, nil
}

func ProcessPaymentActivity(ctx context.Context, amount float64) (string, error) {
    // Payment gateway integration
    return "payment_" + uuid.NewString(), nil
}

func ShipOrderActivity(ctx context.Context, orderID, paymentID string) error {
    // Shipping service integration
    log.Printf("Shipping order %s (payment: %s)", orderID, paymentID)
    return nil
}
```

### 3. Set up Worker and Backend

```go
func main() {
    ctx := context.Background()

    // Choose your backend
    backend := sqlite.NewSqliteBackend("workflows.db")
    
    // Create and start worker
    worker := worker.New(backend, nil)
    worker.RegisterWorkflow(OrderProcessingWorkflow)
    worker.RegisterActivity(ValidateOrderActivity)
    worker.RegisterActivity(ProcessPaymentActivity)
    worker.RegisterActivity(ShipOrderActivity)
    
    go func() {
        if err := worker.Start(ctx); err != nil {
            log.Fatal("Failed to start worker:", err)
        }
    }()

    // Create client and start workflow
    client := client.New(backend)
    
    wf, err := client.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
        InstanceID: "order-12345",
    }, OrderProcessingWorkflow, "order-12345")
    if err != nil {
        log.Fatal("Failed to start workflow:", err)
    }

    // Wait for completion
    err = client.GetWorkflowResult[any](ctx, client, wf, 30*time.Second)
    if err != nil {
        log.Fatal("Workflow failed:", err)
    }
    
    log.Println("Order processed successfully!")
}
```

### 4. Run Your Workflow

```bash
go run main.go
```

That's it! Your workflow is now running with full durability and fault tolerance.

---

## Core Concepts

### Workflows
- **Deterministic**: Must produce the same result given the same input
- **Durable**: Survive process crashes and restarts
- **Versioned**: Support for backward-compatible changes

### Activities  
- **Side Effects**: Database calls, API requests, file operations
- **Automatic Retries**: Configurable retry policies
- **Timeouts**: Built-in timeout handling

### Workers
- **Execution Engine**: Processes workflows and activities
- **Horizontal Scaling**: Multiple workers can share the workload
- **Queue Management**: Support for task routing and prioritization

---

## Examples

Explore comprehensive examples in the [`samples/`](./samples) directory:

| Example | Description |
|---------|-------------|
| **[Simple](./samples/simple)** | Basic workflow with activities |
| **[Concurrent](./samples/concurrent)** | Parallel activity execution |
| **[Sub-workflows](./samples/subworkflow)** | Workflow composition patterns |
| **[Signals](./samples/signal)** | External event handling |
| **[Timers](./samples/timer)** | Scheduled and delayed execution |
| **[Error Handling](./samples/errors)** | Comprehensive error scenarios |
| **[Web Dashboard](./samples/web)** | Built-in diagnostic UI |
| **[Continue-as-New](./samples/continue-as-new)** | Long-running workflow patterns |

### Try the Examples

```bash
# Simple workflow with SQLite
cd samples/simple && go run . -backend sqlite

# Web dashboard with diagnostic UI
cd samples/web && go run .
# Open http://localhost:3000/diag

# Benchmark utility
cd bench && go run .
```

---

## Backends

Choose the storage backend that fits your needs:

| Backend | Use Case | Pros | Cons |
|---------|----------|------|------|
| **SQLite** | Development, small deployments | Zero setup, embedded | Single instance |
| **MySQL** | Production, high availability | Proven, scalable | Requires setup |
| **Redis** | High performance, caching | Very fast, simple | Memory-based |
| **In-Memory** | Testing, development | No persistence needed | Data loss on restart |

### Backend Examples

```go
// SQLite - Great for development and small deployments
backend := sqlite.NewSqliteBackend("workflows.db")

// MySQL - Production-ready with high availability
backend := mysql.NewMysqlBackend("localhost", 3306, "user", "pass", "workflows_db")

// Redis - High performance for large-scale deployments  
backend := redis.NewRedisBackend(&redis.Options{Addr: "localhost:6379"})

// In-Memory - Perfect for testing
backend := monoprocess.NewMonoprocessBackend()
```

---

## Advanced Features

### 🔄 Workflow Orchestrator (Simplified API)

For simpler use cases, use the unified orchestrator that combines client and worker:

```go
orchestrator := worker.NewWorkflowOrchestrator(backend, nil)
orchestrator.RegisterWorkflow(MyWorkflow)
orchestrator.RegisterActivity(MyActivity)

if err := orchestrator.Start(ctx); err != nil {
    panic(err)
}

wf, err := orchestrator.CreateWorkflowInstance(ctx, 
    client.WorkflowInstanceOptions{InstanceID: uuid.NewString()}, 
    MyWorkflow, "input")

result, err := client.GetWorkflowResult[string](ctx, orchestrator.Client, wf, 5*time.Second)
```

### 📊 Observability

Built-in support for OpenTelemetry tracing and metrics:

```go
backend := sqlite.NewSqliteBackend("workflows.db",
    backend.WithTracerProvider(tracerProvider),
    backend.WithMetrics(metricsClient),
)
```

### 🧪 Testing

Comprehensive testing utilities for both workflows and activities:

```go
func TestMyWorkflow(t *testing.T) {
    tester := tester.NewWorkflowTester[string](MyWorkflow)
    tester.OnActivity(MyActivity, mock.Anything).Return("result", nil)
    
    result, err := tester.Execute("input")
    require.NoError(t, err)
    assert.Equal(t, "expected", result)
}
```

---

## Documentation

### 📚 Complete Documentation
Visit **[cschleiden.github.io/go-workflows](http://cschleiden.github.io/go-workflows)** for comprehensive documentation including:

- Detailed API reference
- Advanced patterns and best practices  
- Backend configuration guides
- Migration and deployment strategies

### 📝 Blog Posts
- [Building Durable Workflows in Go - Part 1](https://cschleiden.dev/blog/2022-02-13-go-workflows-part1/)
- [Building Durable Workflows in Go - Part 2](https://cschleiden.dev/blog/2022-05-02-go-workflows-part2/)

---

## Contributing

We welcome contributions! See our [development guide](./DEVELOPMENT.md) for setup instructions.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/cschleiden/go-workflows.git
cd go-workflows

# Install dependencies
go mod download

# Start development services
docker compose up -d

# Run tests
go test -short ./...

# Build project
go build ./...
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">
  <sub>Built with ❤️ by <a href="https://github.com/cschleiden">Christopher Schleiden</a></sub>
</div>
