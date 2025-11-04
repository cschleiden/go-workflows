# PostgreSQL LISTEN/NOTIFY Sample

This sample demonstrates the use of PostgreSQL's LISTEN/NOTIFY feature for reactive task polling in the go-workflows library.

## Overview

By default, the PostgreSQL backend polls for new tasks at regular intervals. With LISTEN/NOTIFY enabled, workers are notified immediately when new tasks become available, resulting in:

- **Faster task execution**: Workers respond immediately to new work
- **Reduced database load**: Less polling queries
- **Better scalability**: More efficient use of database connections

## Running the Sample

### Without LISTEN/NOTIFY (traditional polling):
```bash
go run .
```

### With LISTEN/NOTIFY enabled:
```bash
go run . -notify
```

## Expected Results

You should observe significantly faster workflow execution times when LISTEN/NOTIFY is enabled:

- **Traditional polling**: ~500-800ms (depends on polling interval)
- **LISTEN/NOTIFY**: ~50-150ms (near-instant notification)

## How It Works

1. **Database Triggers**: When new tasks are inserted into `pending_events` or `activities` tables, PostgreSQL triggers send NOTIFY messages
2. **Listener Connections**: The backend maintains dedicated LISTEN connections to PostgreSQL
3. **Immediate Wake-up**: When a notification is received, waiting workers immediately check for new tasks
4. **Fallback**: If no notification is received, workers still poll periodically as a safety mechanism

## Enabling in Your Application

```go
import (
    "github.com/cschleiden/go-workflows/backend"
    "github.com/cschleiden/go-workflows/backend/postgres"
)

b := postgres.NewPostgresBackend(
    "localhost", 5432, "user", "password", "database",
    postgres.WithNotifications(true),
    postgres.WithBackendOptions(backend.WithStickyTimeout(0)),
)
```

## Performance Characteristics

- **Latency**: Near-instant task notification vs polling interval delay
- **Throughput**: Higher task processing rate with immediate notification
- **Resource Usage**: Requires one additional PostgreSQL connection per backend instance for listening

## When to Use

LISTEN/NOTIFY is recommended for:
- Production deployments where latency matters
- High-throughput scenarios
- Applications with sporadic workflow execution

Traditional polling may be sufficient for:
- Development/testing environments
- Low-frequency workflow execution
- Scenarios where additional connections are a concern
