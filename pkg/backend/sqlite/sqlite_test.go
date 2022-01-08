package sqlite

import (
	"testing"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/test"
)

func Test_SqliteBackend(t *testing.T) {
	test.TestBackend(t, test.Tester{
		New: func() backend.Backend {
			return NewSqliteBackend("test.db?mode=memory")
		},
	})
}
