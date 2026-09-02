# PostgreSQL backend

## Features

### LISTEN/NOTIFY for Reactive Task Polling

The PostgreSQL backend supports reactive task polling using PostgreSQL's LISTEN/NOTIFY feature. When enabled, workers are notified immediately when new tasks become available, instead of polling at regular intervals.

**Benefits:**
- **Faster task execution**: Workers respond immediately to new work
- **Reduced database load**: Fewer polling queries
- **Better scalability**: More efficient use of database connections

**Usage:**
```go
import (
    "github.com/cschleiden/go-workflows/backend/postgres"
)

b := postgres.NewPostgresBackend(
    "localhost", 5432, "user", "password", "database",
    postgres.WithNotifications(true),  // Enable LISTEN/NOTIFY
)
```

**How it works:**
1. Database triggers on `pending_events` and `activities` tables send NOTIFY messages when new tasks are inserted
2. The backend maintains dedicated LISTEN connections to PostgreSQL
3. When a notification is received, waiting workers immediately check for new tasks
4. Falls back to traditional polling if no notification is received

See the [postgres-notify sample](../../samples/postgres-notify) for a working example.

## Adding a migration

1. Install [golang-migrate/migrate](https://github.com/golang-migrate/migrate)
1. ```bash
   migrate create -ext sql -dir ./db/migrations -seq <name>
   ```
