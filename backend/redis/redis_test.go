package redis

import (
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
)

func Test_RedisBackend(t *testing.T) {
	test.TestBackend(t, test.Tester{
		New: func() backend.Backend {
			// Disable sticky workflow behavior for the test execution
			return NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)
		},
	})
}
