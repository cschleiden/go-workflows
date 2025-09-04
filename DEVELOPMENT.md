## Setup dependencies for development and tests

1. `docker-compose up`

## Build

```bash
make build
```

## Test

```bash
make test
```

## Lint

Install [golangci-lint](https://golangci-lint.run/usage/install/) and run

```bash
make lint
```

### Use custom linter

1. Build analyzer `go build -tags analyzerplugin -buildmode=plugin analyzer/plugin/plugin.go`
