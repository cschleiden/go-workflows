package samples

import (
	"flag"
	"time"

	redisv8 "github.com/go-redis/redis/v8"
	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/backend/mysql"
	"github.com/ticctech/go-workflows/backend/redis"
	"github.com/ticctech/go-workflows/backend/sqlite"
)

func GetBackend(name string, opt ...backend.BackendOption) backend.Backend {
	b := flag.String("backend", "redis", "backend to use: memory, sqlite, mysql, redis")
	flag.Parse()

	switch *b {
	case "memory":
		return sqlite.NewInMemoryBackend(opt...)

	case "sqlite":
		return sqlite.NewSqliteBackend(name+".sqlite", opt...)

	case "mysql":
		return mysql.NewMysqlBackend("localhost", 3306, "root", "root", name, opt...)

	case "redis":
		rclient := redisv8.NewUniversalClient(&redisv8.UniversalOptions{
			Addrs:        []string{"localhost:6379"},
			Username:     "",
			Password:     "RedisPassw0rd",
			DB:           0,
			WriteTimeout: time.Second * 30,
			ReadTimeout:  time.Second * 30,
		})
		b, err := redis.NewRedisBackend(rclient, redis.WithBackendOptions(opt...))
		if err != nil {
			panic(err)
		}

		return b

	default:
		panic("unknown backend " + *b)
	}
}
