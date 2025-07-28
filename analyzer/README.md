## Custom analyzer

This package implements a basic analyzer for checking various common workflow error conditions.

In your own .golangci.yaml configuration file, you can enable it like this:

```yaml
version: "2"

linters:
  enable:
    - goworkflows

  settings:
    custom:
      goworkflows:
        type: module
        original-url: github.com/cschleiden/go-workflows/analyzer
```