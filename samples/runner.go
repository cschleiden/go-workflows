package samples

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/mysql"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/diag"
	redisv9 "github.com/redis/go-redis/v9"
)

func GetBackend(name string, opt ...backend.BackendOption) backend.Backend {
	b := flag.String("backend", "redis", "backend to use: memory, sqlite, mysql, redis")
	flag.Parse()

	switch *b {
	case "memory":
		return sqlite.NewInMemoryBackend(sqlite.WithBackendOptions(opt...))

	case "sqlite":
		return sqlite.NewSqliteBackend(name+".sqlite", sqlite.WithBackendOptions(opt...))

	case "mysql":
		return mysql.NewMysqlBackend("localhost", 3306, "root", "root", name, opt...)

	case "redis":
		rclient := redisv9.NewUniversalClient(&redisv9.UniversalOptions{
			Addrs:        []string{"localhost:6379"},
			Username:     "",
			Password:     "RedisPassw0rd",
			DB:           0,
			WriteTimeout: time.Second * 30,
			ReadTimeout:  time.Second * 30,
		})

		rclient.FlushAll(context.Background()).Result()

		b, err := redis.NewRedisBackend(rclient, redis.WithBackendOptions(opt...))
		if err != nil {
			panic(err)
		}

		// Start diagnostic server under /diag
		m := http.NewServeMux()
		m.Handle("/diag/", http.StripPrefix("/diag", diag.NewServeMux(b)))
		go http.ListenAndServe(":3000", m)

		log.Println("Debug UI available at http://localhost:3000/diag")

		return b

	default:
		panic("unknown backend " + *b)
	}
}
