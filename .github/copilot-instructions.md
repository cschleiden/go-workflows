# go-workflows Development Instructions

Durable workflows library for Go that borrows heavily from Temporal and DTFx. Supports multiple backends (MySQL, Redis, SQLite, in-memory) and provides comprehensive workflow orchestration capabilities.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap and Build
Run these commands in sequence to set up a working development environment:

```bash
# 1. Download Go dependencies - NEVER CANCEL: Takes ~3.5 minutes. Set timeout to 300+ seconds.
go mod download

# 2. Build all packages - Takes ~40 seconds. Set timeout to 120+ seconds.
go build -v ./...

# 3. Start development dependencies - Takes ~13 seconds. Set timeout to 60+ seconds.
docker compose up -d
```

### Testing
```bash
# Run short tests - NEVER CANCEL: Takes ~45 seconds. Set timeout to 120+ seconds.
go test -short -timeout 120s -race -count 1 -v ./...

# Run full test suite - NEVER CANCEL: Takes ~2.5 minutes. Set timeout to 300+ seconds.
go test -timeout 180s -race -count 1 -v ./...

# Install test reporting tool
go install github.com/jstemmer/go-junit-report/v2@latest

# Run tests with JUnit output (as used in CI) - NEVER CANCEL: Takes ~2.5 minutes. Set timeout to 300+ seconds.
go test -timeout 120s -race -count 1 -v ./... 2>&1 | go-junit-report -set-exit-code -iocopy -out "report.xml"
```

### Linting
```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Build custom analyzer plugin - Takes ~8 seconds. Set timeout to 60+ seconds.
go build -tags analyzerplugin -buildmode=plugin analyzer/plugin/plugin.go

# Run basic linting (custom analyzer has version conflicts) - Takes ~12 seconds. Set timeout to 120+ seconds.
~/go/bin/golangci-lint run --disable-all --enable=gofmt,govet,ineffassign,misspell --timeout=5m
```

### Run Sample Applications
```bash
# Simple workflow example with Redis backend (default)
cd samples/simple && go run .

# Simple workflow with SQLite backend  
cd samples/simple && go run . -backend sqlite

# Benchmark utility
cd bench && go run .

# Web example with diagnostic UI
cd samples/web && go run .
# Access UI at http://localhost:3000/diag
```

## Validation

### Core Workflow Functionality
Always test core workflow functionality after making changes:

1. **Simple End-to-End Test:**
   ```bash
   cd samples/simple && go run . -backend sqlite
   ```
   Expected output: "Workflow finished. Result: 59"

2. **Multi-Backend Test:**
   ```bash
   # Test Redis backend (requires docker compose up)
   cd samples/simple && go run .
   
   # Test SQLite backend (embedded)  
   cd samples/simple && go run . -backend sqlite
   ```

3. **Benchmark Validation:**
   ```bash
   cd bench && go run .
   ```
   Expected: Workflow hierarchy execution with metrics output

### Build Validation
Always run these validation steps before committing:

1. **Full Build:** `go build -v ./...` - Must complete without errors
2. **Short Tests:** `go test -short -timeout 120s -race -count 1 -v ./...` - Should pass (1 known non-critical test failure in tester package)
3. **Sample Execution:** At least one sample must run successfully
4. **Basic Linting:** Check for obvious issues with basic linters

### Manual Testing Scenarios
Execute these scenarios to verify workflow functionality:

1. **Basic Activity Workflow:**
   - Run `samples/simple`
   - Verify workflow executes activities and returns result
   - Check no errors in output

2. **Subworkflow Execution:**
   - Run `bench` utility
   - Verify hierarchical workflow creation and execution
   - Check metrics output shows expected activity and workflow counts

3. **Backend Switching:**
   - Test same workflow with different backends (Redis, SQLite)
   - Verify consistent behavior across backends

## Repository Structure

### Key Directories
- **`/samples/`** - Example applications demonstrating workflow patterns:
  - `simple/` - Basic workflow with activities
  - `web/` - Web UI example with diagnostics 
  - `subworkflow/` - Nested workflow patterns
  - `timer/` - Timer and scheduling examples
  - `signal/` - Signal handling patterns
- **`/backend/`** - Storage backend implementations:
  - `mysql/` - MySQL backend
  - `redis/` - Redis backend  
  - `sqlite/` - SQLite backend
  - `monoprocess/` - In-memory backend
- **`/workflow/`** - Core workflow execution engine
- **`/activity/`** - Activity execution framework
- **`/tester/`** - Testing utilities for workflow development
- **`/client/`** - Client library for workflow interactions
- **`/worker/`** - Worker process implementation
- **`/analyzer/`** - Custom Go analyzer for workflow code validation

### Build and Configuration Files
- **`go.mod`** - Go module definition with all dependencies
- **`docker-compose.yml`** - Development dependencies (MySQL, Redis)
- **`.golangci.yml`** - Linting configuration (includes custom analyzer)
- **`tools.go`** - Development tool dependencies
- **`.github/workflows/go.yml`** - CI pipeline definition

### Development Dependencies
The project requires Docker services for full functionality:

```bash
# Start MySQL and Redis services
docker compose up -d

# Check service status
docker compose ps

# View logs
docker compose logs

# Stop services
docker compose down
```

## Common Tasks

### Adding New Workflow Examples
1. Create directory under `samples/`
2. Implement workflow and activity functions
3. Add backend selection similar to existing samples
4. Test with multiple backends
5. Add to documentation

### Backend Development
1. Implement `backend.Backend` interface
2. Add database schema/migrations as needed
3. Create integration tests using `backend/test` package
4. Add CI job in `.github/workflows/go.yml`

### Custom Analyzer Rules
1. Modify `analyzer/analyzer.go`
2. Rebuild plugin: `go build -tags analyzerplugin -buildmode=plugin analyzer/plugin/plugin.go`
3. Test with golangci-lint (note: version compatibility issues exist)

## Timing Expectations

| Operation | Time | Timeout Recommendation |
|-----------|------|----------------------|
| `go mod download` | ~3.5 minutes | 300+ seconds |
| `go build -v ./...` | ~40 seconds | 120+ seconds |
| `go test -short` | ~45 seconds | 120+ seconds |  
| `go test` (full) | ~2.5 minutes | 300+ seconds |
| `docker compose up -d` | ~13 seconds | 60+ seconds |
| Plugin build | ~8 seconds | 60+ seconds |
| Linting | ~12 seconds | 120+ seconds |

**CRITICAL:** NEVER CANCEL build or test operations. Always wait for completion and use recommended timeout values.

## Known Issues
- Custom analyzer plugin has version conflicts with current golangci-lint
- One test failure in `tester/tester_timers_test.go` (non-critical)  
- Various linter warnings present but not blocking

## CI Pipeline
The GitHub Actions workflow (`.github/workflows/go.yml`) runs:
1. Build validation
2. Short tests on main branch
3. Backend-specific tests (Redis, MySQL, SQLite) in parallel
4. Test reporting with summaries

Always ensure your changes pass the same tests that CI runs.