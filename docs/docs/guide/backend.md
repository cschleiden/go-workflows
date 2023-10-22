---
sidebar_position: 4
---

# Backend instance

The backend is responsible for persisting the workflow events. Currently there is an in-memory backend implementation for testing, one using [SQLite](http://sqlite.org), one using MySql, and one using Redis.

```go
b := sqlite.NewSqliteBackend("simple.sqlite")
```