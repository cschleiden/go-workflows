# Sqlite backend

## Adding a migration

1. Install [golang-migrate/migrate](https://www.github.com/golang-migrate/migrate)
1. For sqlite support you might want to install it via ```
go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```
1. ```bash
   migrate create -ext sql -dir ./db/migrations -seq <name>
   ```