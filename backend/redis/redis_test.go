package redis

import (
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/stretchr/testify/require"
)

func Test_RedisBackend(t *testing.T) {
	test.TestBackend(t, test.Tester{
		New: func() backend.Backend {
			// Disable sticky workflow behavior for the test execution
			b, err := NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0,
				WithBlockTimeout(time.Millisecond*2))
			require.NoError(t, err)
			return b
		},
	})
}
