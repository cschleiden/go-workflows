# go-workflows Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint
CUSTOM_GOLINT=./custom-gcl

# Test parameters
TEST_TIMEOUT=240s
TEST_FLAGS=-race -count=1 -v
TEST_SHORT_FLAGS=-short $(TEST_FLAGS)

# Build targets
.PHONY: all build clean test test-integration test-redis test-mysql test-sqlite test-backends lint fmt

# Default target
all: build test lint

# Build the main project
build:
	@echo "Building go-workflows..."
	$(GOBUILD) -v ./...

# Run tests with short flag (excludes long-running tests)
test:
	@echo "Running tests..."
	$(GOTEST) $(TEST_SHORT_FLAGS) -timeout $(TEST_TIMEOUT) ./...

# Run all tests
test-integration:
	@echo "Running all tests..."
	$(GOTEST) $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) ./...

# Run Redis backend tests
test-redis:
	@echo "Running Redis backend tests..."
	$(GOTEST) $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) github.com/cschleiden/go-workflows/backend/redis

# Run MySQL backend tests
test-mysql:
	@echo "Running MySQL backend tests..."
	$(GOTEST) $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) github.com/cschleiden/go-workflows/backend/mysql

# Run SQLite backend tests
test-sqlite:
	@echo "Running SQLite backend tests..."
	$(GOTEST) $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) github.com/cschleiden/go-workflows/backend/sqlite

# Run monoprocess backend tests
test-monoprocess:
	@echo "Running monoprocess backend tests..."
	$(GOTEST) $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) github.com/cschleiden/go-workflows/backend/monoprocess

# Run all backend tests
test-backends: test-redis test-mysql test-sqlite test-monoprocess

custom-gcl:
	@echo "Checking if golangci-lint is installed..."
	@which $(GOLINT) > /dev/null || (echo "golangci-lint is not installed. Please install it first." && exit 1)
	@echo "Building custom linter plugin..."
	$(GOLINT) custom

# Lint the code
lint: custom-gcl
	@echo "Running linter..."
	$(CUSTOM_GOLINT) run

# Format the code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f coverage.out coverage.html
	rm -f *.sqlite
	rm -f report.xml

