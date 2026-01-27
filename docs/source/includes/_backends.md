# Backends

There are four backend implementations maintained in this repository. Some backend implementations have custom options and all of them accept:

- `WithStickyTimeout(timeout time.Duration)` - Set the timeout for sticky tasks. Defaults to 30 seconds
- `WithWorkflowLockTimeout(timeout time.Duration)` - Set the timeout for workflow task locks. Defaults to 1 minute
- `WithActivityLockTimeout(timeout time.Duration)` - Set the timeout for activity task locks. Defaults to 2 minutes
- `WithLogger(logger *slog.Logger)` - Set the logger implementation
- `WithMetrics(client metrics.Client)` - Set the metrics client
- `WithTracerProvider(tp trace.TracerProvider)` - Set the OpenTelemetry tracer provider
- `WithConverter(converter converter.Converter)` - Provide a custom `Converter` implementation
- `WithContextPropagator(prop workflow.ContextPropagator)` - Adds a custom context propagator
- `WithRemoveContinuedAsNewInstances()` - Immediately removes workflow instances that complete using ContinueAsNew, including their history. ContinueAsNew allows workflows to restart with new parameters while preserving the same instance ID. By default, such instances are retained according to the configured retention policy. Use this option to prevent storage bloat in workflows that frequently use ContinueAsNew.


## SQLite

```go
func NewSqliteBackend(path string, opts ...option)
```

Create a new SQLite backend instance with `NewSqliteBackend`.

### Using an Existing Connection

```go
func NewSqliteBackendWithDB(db *sql.DB, opts ...option)
```

If you already have a `*sql.DB` connection, you can use `NewSqliteBackendWithDB` to create a backend that uses your existing connection. When using this constructor:

- The backend will **not** close the database connection when `Close()` is called
- Migrations are **disabled by default** - use `WithApplyMigrations(true)` to enable them
- You are responsible for configuring the connection appropriately (e.g., WAL mode, busy timeout, max open connections)

```go
db, _ := sql.Open("sqlite", "file:mydb.sqlite?_txlock=immediate")
db.Exec("PRAGMA journal_mode=WAL;")
db.Exec("PRAGMA busy_timeout = 5000;")
db.SetMaxOpenConns(1)

backend := sqlite.NewSqliteBackendWithDB(db, sqlite.WithApplyMigrations(true))
```

### Options

- `WithApplyMigrations(applyMigrations bool)` - Set whether migrations should be applied on startup. Defaults to `true` for `NewSqliteBackend`, `false` for `NewSqliteBackendWithDB`
- `WithBackendOptions(opts ...backend.BackendOption)` - Apply generic backend options

### Schema

See `migrations/sqlite` for the schema and migrations. Main tables:

- `instances` - Tracks workflow instances. Functions as instance queue joined with `pending_events`
- `pending_events` - Pending events for workflow instances
- `history` - History for workflow instances
- `activities` - Queue of pending activities
- `attributes` - Payloads of events

## MySQL

```go
func NewMysqlBackend(host string, port int, user, password, database string, opts ...option)
```

Create a new MySQL backend instance with `NewMysqlBackend`.

### Using an Existing Connection

```go
func NewMysqlBackendWithDB(db *sql.DB, opts ...option)
```

If you already have a `*sql.DB` connection, you can use `NewMysqlBackendWithDB` to create a backend that uses your existing connection. When using this constructor:

- The backend will **not** close the database connection when `Close()` is called
- Migrations are **disabled by default**
- To enable migrations, you must provide a DSN with `WithMigrationDSN()` that supports multi-statement queries

```go
db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/mydb?parseTime=true")

// To enable migrations, provide a DSN with multiStatements=true
migrationDSN := "user:pass@tcp(localhost:3306)/mydb?parseTime=true&multiStatements=true"

backend := mysql.NewMysqlBackendWithDB(db,
    mysql.WithApplyMigrations(true),
    mysql.WithMigrationDSN(migrationDSN),
)
```

### Options

- `WithMySQLOptions(f func(db *sql.DB))` - Apply custom options to the MySQL database connection
- `WithApplyMigrations(applyMigrations bool)` - Set whether migrations should be applied on startup. Defaults to `true` for `NewMysqlBackend`, `false` for `NewMysqlBackendWithDB`
- `WithMigrationDSN(dsn string)` - Set the DSN to use for migrations. Required when using `NewMysqlBackendWithDB` with `ApplyMigrations` enabled. The DSN must support multi-statement queries.
- `WithBackendOptions(opts ...backend.BackendOption)` - Apply generic backend options


### Schema

See `migrations/mysql` for the schema and migrations. Main tables:

- `instances` - Tracks workflow instances. Functions as instance queue joined with `pending_events`
- `pending_events` - Pending events for workflow instances
- `history` - History for workflow instances
- `activities` - Queue of pending activities
- `attributes` - Payloads of events

## PostgreSQL

```go
func NewPostgresBackend(host string, port int, user, password, database string, opts ...option)
```

Create a new PostgreSQL backend instance with `NewPostgresBackend`.

### Using an Existing Connection

```go
func NewPostgresBackendWithDB(db *sql.DB, opts ...option)
```

If you already have a `*sql.DB` connection, you can use `NewPostgresBackendWithDB` to create a backend that uses your existing connection. When using this constructor:

- The backend will **not** close the database connection when `Close()` is called
- Migrations are **disabled by default** - use `WithApplyMigrations(true)` to enable them

```go
db, _ := sql.Open("pgx", "host=localhost port=5432 user=myuser password=mypass dbname=mydb sslmode=disable")

backend := postgres.NewPostgresBackendWithDB(db, postgres.WithApplyMigrations(true))
```

### Options

- `WithPostgresOptions(f func(db *sql.DB))` - Apply custom options to the PostgreSQL database connection
- `WithApplyMigrations(applyMigrations bool)` - Set whether migrations should be applied on startup. Defaults to `true` for `NewPostgresBackend`, `false` for `NewPostgresBackendWithDB`
- `WithBackendOptions(opts ...backend.BackendOption)` - Apply generic backend options


### Schema

See `migrations/postgres` for the schema and migrations. Main tables:

- `instances` - Tracks workflow instances. Functions as instance queue joined with `pending_events`
- `pending_events` - Pending events for workflow instances
- `history` - History for workflow instances
- `activities` - Queue of pending activities
- `attributes` - Payloads of events

## Redis

```go
func NewRedisBackend(client redis.UniversalClient, opts ...RedisBackendOption)
```

Create a new Redis backend instance with `NewRedisBackend`.

### Options

- `WithKeyPrefix` - Set the key prefix for all keys. Defaults to `""`
- `WithBlockTimeout(timeout time.Duration)` - Set the timeout for blocking operations. Defaults to `5s`
- `WithAutoExpiration(expireFinishedRunsAfter time.Duration)` - Set the expiration time for finished runs. Defaults to `0`, which never expires runs
- `WithAutoExpirationContinueAsNew(expireContinuedAsNewRunsAfter time.Duration)` - Set the expiration time for continued as new runs. Defaults to `0`, which uses the same value as `WithAutoExpiration`
- `WithBackendOptions(opts ...backend.BackendOption)` - Apply generic backend options


### Schema/Keys

Shared keys:

- `instances-by-creation` - `ZSET` - Instances sorted by creation time
- `instances-active` - `SET` - Active instances
- `instances-expiring` - `SET` - Instances about to expire

- `task-queue:workflows` - `STREAM` - Task queue for workflows
- `task-queue:activities` - `STREAM` - Task queue for activities

Instance specific keys:

- `active-instance-execution:{instanceID}` - Latest execution for a workflow instance
- `instance:{instanceID}:{executionID}` - State of the workflow instance
- `pending-events:{instanceID}:{executionID}` - `STREAM` - Pending events for a workflow instance
- `history:{instanceID}:{executionID}` - `STREAM` - History for a workflow instance
- `payload:{instanceID}:{executionID}` - `HASH` - Payloads of events for given workflow instance

- `future-events` - `ZSET` - Events not yet visible like timer events



## Custom implementation

To provide a custom backend, implement the following interface:

```golang
type Backend interface {
	// CreateWorkflowInstance creates a new workflow instance
	CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error

	// CancelWorkflowInstance cancels a running workflow instance
	CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, cancelEvent *history.Event) error

	// RemoveWorkflowInstance removes a workflow instance
	RemoveWorkflowInstance(ctx context.Context, instance *workflow.Instance) error

	// GetWorkflowInstanceState returns the state of the given workflow instance
	GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error)

	// GetWorkflowInstanceHistory returns the workflow history for the given instance. When lastSequenceID
	// is given, only events after that event are returned. Otherwise the full history is returned.
	GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]*history.Event, error)

	// SignalWorkflow signals a running workflow instance
	//
	// If the given instance does not exist, it will return an error
	SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error

	// GetWorkflowTask returns a pending workflow task or nil if there are no pending workflow executions
	GetWorkflowTask(ctx context.Context) (*WorkflowTask, error)

	// ExtendWorkflowTask extends the lock of a workflow task
	ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error

	// CompleteWorkflowTask checkpoints a workflow task retrieved using GetWorkflowTask
	//
	// This checkpoints the execution. events are new events from the last workflow execution
	// which will be added to the workflow instance history. workflowEvents are new events for the
	// completed or other workflow instances.
	CompleteWorkflowTask(
		ctx context.Context, task *WorkflowTask, instance *workflow.Instance, state core.WorkflowInstanceState,
		executedEvents, activityEvents, timerEvents []*history.Event, workflowEvents []history.WorkflowEvent) error

	// GetActivityTask returns a pending activity task or nil if there are no pending activities
	GetActivityTask(ctx context.Context) (*ActivityTask, error)

	// CompleteActivityTask completes an activity task retrieved using GetActivityTask
	CompleteActivityTask(ctx context.Context, instance *workflow.Instance, activityID string, event *history.Event) error

	// ExtendActivityTask extends the lock of an activity task
	ExtendActivityTask(ctx context.Context, activityID string) error

	// GetStats returns stats about the backend
	GetStats(ctx context.Context) (*Stats, error)

	// Logger returns the configured logger for the backend
	Logger() *slog.Logger

	// Tracer returns the configured trace provider for the backend
	Tracer() trace.Tracer

	// Metrics returns the configured metrics client for the backend
	Metrics() metrics.Client

	// Converter returns the configured converter for the backend
	Converter() converter.Converter

	// ContextPropagators returns the configured context propagators for the backend
	ContextPropagators() []workflow.ContextPropagator

	// Close closes any underlying resources
	Close() error
}
```