# Simple benchmark utility

The benchmark runs a "workflow tree" made up of root, mid and leaf workflows. The root workflows start mid workflows, which in turn start leaf workflows. The leaf workflows execute activities.

Options to influence the number of workflows, sub-workflows, and activities:

- `-runs` Number of root workflows to start
- `-fanout` Number of child "mid" workflows to execute per root/mid workflow
- `-leaffanout` Number of leaf workflows to execute per mid workflow
- `-depth` Depth of mid workflows

```
                          ┌──────┐             ──────┐
                          │ Root │                   │
                          └──┬───┘                   │
                             │                       │
fanout          ────► ┌──────┴───────┐               │
                      │              │               │
                   ┌──┴──┐        ┌──┴──┐            │ depth
                   │ Mid │        │ Mid │            │
                   └──┬──┘        └──┬──┘            │
                      │              │               │
leaffanout ───► ┌─────┴──┐        ┌──┴─────┐         │
                │        │        │        │         │
            ┌───┴┐    ┌──┴─┐   ┌──┴─┐   ┌──┴─┐       │
            │Leaf│    │Leaf│   │Leaf│   │Leaf│       │
            └────┘    └────┘   └────┘   └────┘    ───┘

         ┌────────────┐         ┌───┐
         │ Activities │   ...   │   │
         └────────────┘         └───┘
```

## Run

```shell
$ go run .
```

### Help

```shell
$ go run .h -h
  -activities int
        Number of activities to execute per leaf workflow (default 2)
  -backend string
        Backend to use. Supported backends are:
        - redis
        - mysql
        - sqlite
        - postgres
         (default "redis")
  -cachesize int
        Size of the workflow executor cache (default 128)
  -depth int
        Depth of mid workflows (default 2)
  -fanout int
        Number of child workflows to execute per root/mid workflow (default 2)
  -format string
        Output format. Supported formats are:
        - text
        - csv
         (default "text")
  -leaffanout int
        Number of leaf workflows to execute per mid workflow (default 2)
  -resultsize int
        Size of activity result payload in bytes (default 100)
  -runs int
        Number of root workflows to start (default 1)
  -scenario string
        Scenario to run. Support scenarios are:
        - basic
         (default "basic")
  -timeout duration
        Timeout for the benchmark run (default 30s)
```