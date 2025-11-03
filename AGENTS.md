# go-workflows Development Guide for AI Agents

Durable workflows library for Go that borrows heavily from Temporal and DTFx. Supports multiple backends (MySQL, Redis, SQLite, in-memory) and provides comprehensive workflow orchestration capabilities.

This guide provides essential information for AI coding agents working on this project. Always reference this information first before exploring the codebase or running commands.

## Development Environment Setup

### Bootstrap and Build
Execute these commands in sequence to set up a working development environment:

```bash
# 1. Download Go dependencies - Takes ~3.5 minutes. Allow sufficient timeout.
go mod download

# 2. Build all packages - Takes ~40 seconds.
go build -v ./...

# 3. Start development dependencies - Takes ~13 seconds.
docker compose up -d
```

### Testing
```bash
# Run short tests - Takes ~45 seconds.
go test -short -timeout 120s -race -count 1 -v ./...

# Run full test suite - Takes ~2.5 minutes.
go test -timeout 180s -race -count 1 -v ./...

# Install test reporting tool (if needed)
go install github.com/jstemmer/go-junit-report/v2@latest

# Run tests with JUnit output (as used in CI) - Takes ~2.5 minutes.
go test -timeout 120s -race -count 1 -v ./... 2>&1 | go-junit-report -set-exit-code -iocopy -out "report.xml"
```

### Linting

The project uses golangci-lint v2 with a custom analyzer plugin for workflow code validation. There are multiple ways to run the linter:

#### Recommended: Using Makefile (Preferred)
```bash
# Run the full linter suite with custom analyzer - Takes ~12-15 seconds.
make lint
```

This will:
- Check if golangci-lint is installed
- Build the custom analyzer plugin
- Run all configured linters from `.golangci.yml`

#### Manual Setup
If you need to install golangci-lint or run it manually:

```bash
# Install golangci-lint v2 (required for this project)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

# Run the full linter configuration - Takes ~12-15 seconds.
golangci-lint run --timeout=5m
```

**Note:** The project uses golangci-lint v2.4.0 configuration. Version v1.x will not work with the `.golangci.yml` configuration file.

#### Workaround: Basic Linting (When Custom Analyzer Has Issues)
If the custom analyzer has version compatibility issues:

```bash
# Run basic linting without custom analyzer - Takes ~12 seconds.
golangci-lint run --disable-all --enable=gofmt,govet,ineffassign,misspell --timeout=5m
```

#### Available Linters
The `.golangci.yml` configuration enables multiple linters including:
- **Code Quality**: staticcheck, unused, ineffassign, wastedassign
- **Bug Detection**: govet, makezero, prealloc, predeclared
- **Style & Formatting**: gofmt, whitespace, tagalign
- **Testing**: testifylint, tparallel
- **Custom**: goworkflows (workflow-specific validation, currently commented out)

### Sample Applications
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

## Validation and Testing

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

### Pre-Commit Validation
Run these validation steps before committing changes:

1. **Full Build:** `go build -v ./...` - Must complete without errors
2. **Short Tests:** `go test -short -timeout 120s -race -count 1 -v ./...` - Should pass (1 known non-critical test failure in tester package)
3. **Linting:** `make lint` or `golangci-lint run --timeout=5m` - Should pass with no new violations
4. **Code Formatting:** `make fmt` - Ensure code is properly formatted
5. **Sample Execution:** At least one sample must run successfully

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
  - `postgres/` - PostgreSQL backend
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

## Common Development Tasks

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

## Performance and Timing

### Operation Duration Expectations

| Operation | Typical Duration | Notes |
|-----------|-----------------|-------|
| `go mod download` | ~3.5 minutes | Critical operation, allow full completion |
| `go build -v ./...` | ~40 seconds | Full project build |
| `go test -short` | ~45 seconds | Quick test suite |
| `go test` (full) | ~2.5 minutes | Complete test suite |
| `docker compose up -d` | ~13 seconds | Start services |
| Plugin build | ~8 seconds | Custom analyzer |
| Linting | ~12 seconds | Basic linting rules |

**Important:** Allow sufficient time for build and test operations to complete. Interrupting these operations can lead to incomplete or corrupted builds.

## Known Issues and Limitations

### Current Issues
- Custom analyzer plugin has version conflicts with current golangci-lint
- One test failure in `tester/tester_timers_test.go` (non-critical)
- Various linter warnings present but not blocking

### Workarounds
- Use basic linting configuration instead of full analyzer
- Skip non-critical test failures in validation
- Focus on core functionality tests for validation

## CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/go.yml`) performs:
1. Build validation across Go versions
2. Short tests on main branch
3. Backend-specific tests (Redis, MySQL, SQLite) in parallel
4. Test reporting with summaries

Ensure local changes pass the same validation steps that CI runs.

## Best Practices for AI Agents

### Code Modification Guidelines
1. Always run validation tests after making changes
2. Test with multiple backends when modifying core functionality
3. Maintain backward compatibility in API changes
4. Follow existing code patterns and conventions
5. Update relevant documentation and examples

### Error Handling
1. Check for compilation errors after code changes
2. Validate test outcomes, especially for workflow functionality
3. Verify sample applications still work after modifications
4. Monitor for new linter warnings

### Development Workflow
1. Start with understanding the existing codebase structure
2. Make incremental changes and test frequently
3. Use the provided samples to validate functionality
4. Run appropriate validation steps before finalizing changes

## Documentation

The project includes comprehensive documentation built with Slate, a static documentation generator. The documentation is located in the `/docs/` directory and provides detailed guides, API references, and examples.

### Documentation Structure
- **`/docs/source/index.html.md`** - Main documentation file with quickstart guide
- **`/docs/source/includes/`** - Modular documentation sections:
  - `_guide.md` - Comprehensive development guide
  - `_samples.md` - Example applications and use cases
  - `_backends.md` - Backend configuration and usage
  - `_faq.md` - Frequently asked questions
- **`/docs/images/`** - Documentation assets and images
- **`/docs/build/`** - Generated static documentation (created during build)

### Working with Documentation

#### Development Server
Start a local documentation server for live editing:

```bash
cd docs
./serve.sh
```

This will:
- Start a Docker container running Slate
- Mount source files for live reloading
- Serve documentation at http://localhost:4567/

#### Building Documentation
Generate static documentation files:

```bash
cd docs
./build.sh
```

This creates a static site in the `build/` directory suitable for deployment.

#### Documentation Content
The documentation covers:
- **Quickstart Guide** - Basic workflow and activity examples
- **Comprehensive Guide** - Detailed API documentation and patterns
- **Sample Applications** - Real-world usage examples from `/samples/`
- **Backend Configuration** - Setup guides for MySQL, Redis, SQLite backends
- **FAQ Section** - Common questions and troubleshooting

### Documentation Dependencies
The documentation system requires:
- **Docker** - For running the Slate documentation generator
- **Slate Docker Image** (`slatedocs/slate`) - Automatically pulled when needed

### Updating Documentation
When making code changes that affect the public API or add new features:

1. **Update Relevant Sections**: Modify the appropriate `.md` files in `/docs/source/`
2. **Test Locally**: Run `./serve.sh` to preview changes
3. **Validate Examples**: Ensure code examples in documentation are accurate
4. **Build Static Version**: Run `./build.sh` to generate deployable documentation
5. **Update Sample References**: Keep documentation synchronized with `/samples/` directory

### Documentation Best Practices
- Keep code examples in documentation synchronized with working samples
- Test all code snippets to ensure they compile and run correctly
- Update documentation whenever API changes are made
- Include practical examples alongside theoretical explanations
- Maintain consistency between inline code comments and external documentation
